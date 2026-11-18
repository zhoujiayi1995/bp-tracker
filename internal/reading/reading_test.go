package reading

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bptracker/internal/db"
	"bptracker/internal/validate"
)

func ip(v int) *int { return &v }

var shanghai = mustLoc("Asia/Shanghai")

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func TestAverage(t *testing.T) {
	cases := []struct {
		name             string
		items            []Item
		wantSys, wantDia int
		wantPulse        *int
	}{
		{"single", []Item{{130, 85, ip(70)}}, 130, 85, ip(70)},
		{"rounds half up", []Item{{131, 80, ip(70)}, {132, 81, ip(71)}}, 132, 81, ip(71)},
		{"three items", []Item{{140, 90, nil}, {135, 88, nil}, {131, 85, nil}}, 135, 88, nil},
		{"pulse averaged over present values", []Item{{120, 80, ip(60)}, {122, 82, nil}, {124, 84, ip(65)}}, 122, 82, ip(63)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sys, dia, pulse := Average(tc.items)
			if sys != tc.wantSys || dia != tc.wantDia {
				t.Fatalf("Average = %d/%d, want %d/%d", sys, dia, tc.wantSys, tc.wantDia)
			}
			if (pulse == nil) != (tc.wantPulse == nil) || (pulse != nil && *pulse != *tc.wantPulse) {
				t.Fatalf("pulse = %v, want %v", deref(pulse), deref(tc.wantPulse))
			}
		})
	}
}

func TestValidateItems(t *testing.T) {
	cases := []struct {
		name  string
		items []Item
		field string // empty means valid
	}{
		{"valid", []Item{{120, 80, ip(70)}}, ""},
		{"boundaries", []Item{{60, 30, ip(30)}, {260, 160, ip(200)}}, ""},
		{"none", nil, "items"},
		{"too many", []Item{{120, 80, nil}, {120, 80, nil}, {120, 80, nil}, {120, 80, nil}}, "items"},
		{"systolic low", []Item{{59, 40, nil}}, "items[0].systolic"},
		{"systolic high", []Item{{261, 80, nil}}, "items[0].systolic"},
		{"diastolic low", []Item{{120, 29, nil}}, "items[0].diastolic"},
		{"diastolic high", []Item{{200, 161, nil}}, "items[0].diastolic"},
		{"systolic not above diastolic", []Item{{120, 80, ip(70)}, {90, 90, ip(70)}}, "items[1].systolic"},
		{"pulse required", []Item{{120, 80, nil}}, "items[0].pulse"},
		{"pulse range", []Item{{120, 80, ip(201)}}, "items[0].pulse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateItems(tc.items)
			if tc.field == "" {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			var ve *validate.Error
			if !errors.As(err, &ve) || ve.Field != tc.field {
				t.Fatalf("err = %v, want field %s", err, tc.field)
			}
		})
	}
}

func TestInputNormalize(t *testing.T) {
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, shanghai)
	items := []Item{{120, 80, ip(70)}}

	in := Input{Items: items, Note: "  took meds  "}
	if err := in.Normalize(now); err != nil {
		t.Fatal(err)
	}
	if !in.MeasuredAt.Equal(now) || in.Note != "took meds" {
		t.Fatalf("normalized = %+v", in)
	}

	future := Input{Items: items, MeasuredAt: now.Add(time.Hour)}
	var ve *validate.Error
	if err := future.Normalize(now); !errors.As(err, &ve) || ve.Field != "measuredAt" {
		t.Fatalf("future err = %v", err)
	}
	skew := Input{Items: items, MeasuredAt: now.Add(5 * time.Minute)}
	if err := skew.Normalize(now); err != nil {
		t.Fatalf("small clock skew should pass: %v", err)
	}
}

func TestPeriodOf(t *testing.T) {
	cases := []struct {
		t    time.Time
		want Period
	}{
		{time.Date(2026, 11, 17, 0, 0, 0, 0, shanghai), AM},
		{time.Date(2026, 11, 17, 11, 59, 59, 0, shanghai), AM},
		{time.Date(2026, 11, 17, 12, 0, 0, 0, shanghai), PM},
		{time.Date(2026, 11, 17, 17, 59, 0, 0, shanghai), PM},
		{time.Date(2026, 11, 17, 18, 0, 0, 0, shanghai), Evening},
		{time.Date(2026, 11, 17, 23, 59, 0, 0, shanghai), Evening},
		// 03:59 UTC is 11:59 in Shanghai; 04:00 UTC is noon; 10:00 UTC is 18:00.
		{time.Date(2026, 11, 17, 3, 59, 0, 0, time.UTC), AM},
		{time.Date(2026, 11, 17, 4, 0, 0, 0, time.UTC), PM},
		{time.Date(2026, 11, 17, 9, 59, 0, 0, time.UTC), PM},
		{time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC), Evening},
		// 20:00 UTC on the 16th is 04:00 on the 17th in Shanghai.
		{time.Date(2026, 11, 16, 20, 0, 0, 0, time.UTC), AM},
	}
	for _, tc := range cases {
		if got := PeriodOf(tc.t, shanghai); got != tc.want {
			t.Errorf("PeriodOf(%v) = %s, want %s", tc.t, got, tc.want)
		}
	}
}

func TestParseDayRange(t *testing.T) {
	now := time.Date(2026, 11, 17, 22, 30, 0, 0, shanghai)

	r, err := ParseDayRange("", "", shanghai, now, 7, 366)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.From.Format(dateLayout); got != "2026-11-11" {
		t.Fatalf("default from = %s", got)
	}
	start, end := r.Bounds()
	if !start.Equal(time.Date(2026, 11, 11, 0, 0, 0, 0, shanghai)) || !end.Equal(time.Date(2026, 11, 18, 0, 0, 0, 0, shanghai)) {
		t.Fatalf("bounds = %v .. %v", start, end)
	}
	if r.Days() != 7 {
		t.Fatalf("days = %d", r.Days())
	}

	if _, err := ParseDayRange("2026-11-10", "2026-11-01", shanghai, now, 7, 366); err == nil {
		t.Fatal("from after to should fail")
	}
	if _, err := ParseDayRange("2025-01-01", "2026-11-17", shanghai, now, 7, 366); err == nil {
		t.Fatal("range over max should fail")
	}
	if _, err := ParseDayRange("11/01/2026", "", shanghai, now, 7, 366); err == nil {
		t.Fatal("bad format should fail")
	}
}

func TestPeriodBuckets(t *testing.T) {
	now := time.Date(2026, 11, 17, 20, 0, 0, 0, shanghai)
	rng, err := ParseDayRange("2026-11-16", "2026-11-17", shanghai, now, 7, 366)
	if err != nil {
		t.Fatal(err)
	}
	at := func(day, hour int) time.Time { return time.Date(2026, 11, day, hour, 0, 0, 0, shanghai) }
	readings := []Reading{
		{MeasuredAt: at(16, 7), Systolic: 140, Diastolic: 90, Pulse: ip(70)},
		{MeasuredAt: at(16, 9), Systolic: 131, Diastolic: 85, Pulse: nil},
		{MeasuredAt: at(17, 15), Systolic: 128, Diastolic: 82, Pulse: ip(70)},
		{MeasuredAt: at(17, 19), Systolic: 120, Diastolic: 80, Pulse: ip(66)},
	}

	got := PeriodBuckets(readings, rng)
	if len(got) != 6 {
		t.Fatalf("buckets = %d, want 6", len(got))
	}
	// 16 am: avg of 140/90 and 131/85 = 135.5 -> 136, 87.5 -> 88; pulse only from first reading.
	if b := got[0]; b.Date != "2026-11-16" || b.Period != AM || *b.Systolic != 136 || *b.Diastolic != 88 || *b.Pulse != 70 || b.Count != 2 {
		t.Fatalf("16 am = %+v", describe(b))
	}
	if b := got[1]; b.Period != PM || b.Systolic != nil || b.Count != 0 {
		t.Fatalf("16 pm should be empty, got %+v", describe(b))
	}
	if b := got[2]; b.Period != Evening || b.Systolic != nil || b.Count != 0 {
		t.Fatalf("16 evening should be empty, got %+v", describe(b))
	}
	if b := got[3]; b.Date != "2026-11-17" || b.Period != AM || b.Systolic != nil {
		t.Fatalf("17 am should be empty, got %+v", describe(b))
	}
	if b := got[4]; b.Period != PM || *b.Systolic != 128 || *b.Pulse != 70 {
		t.Fatalf("17 pm = %+v", describe(b))
	}
	if b := got[5]; b.Period != Evening || *b.Systolic != 120 || *b.Pulse != 66 {
		t.Fatalf("17 evening = %+v", describe(b))
	}
}

func TestSummarize(t *testing.T) {
	if s := Summarize(nil); s.Count != 0 || s.Systolic != nil {
		t.Fatalf("empty summary = %+v", s)
	}
	s := Summarize([]Reading{{Systolic: 130, Diastolic: 80}, {Systolic: 141, Diastolic: 91, Pulse: ip(72)}})
	if s.Count != 2 || *s.Systolic != 136 || *s.Diastolic != 86 || *s.Pulse != 72 {
		t.Fatalf("summary = %d %d/%d %v", s.Count, *s.Systolic, *s.Diastolic, deref(s.Pulse))
	}
}

func TestStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "bp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	res, _ := d.Exec(`INSERT INTO users (username, password_hash, created_at) VALUES ('alice', 'x', 0)`)
	uid, _ := res.LastInsertId()
	res, _ = d.Exec(`INSERT INTO profiles (display_name, user_id, created_at) VALUES ('Alice', ?, 0)`, uid)
	pid, _ := res.LastInsertId()

	now := time.Date(2026, 11, 17, 9, 0, 0, 0, shanghai)
	in := Input{MeasuredAt: now.Add(-time.Hour), Items: []Item{{130, 85, ip(70)}, {134, 87, ip(74)}}, Note: "morning"}
	id, err := Insert(ctx, d, pid, uid, in, now)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Get(ctx, d, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Systolic != 132 || got.Diastolic != 86 || *got.Pulse != 72 || len(got.Items) != 2 || got.Note != "morning" {
		t.Fatalf("stored = %+v", got)
	}

	in.Items = []Item{{150, 95, nil}}
	if err := Update(ctx, d, id, in); err != nil {
		t.Fatal(err)
	}
	got, _ = Get(ctx, d, id)
	if got.Systolic != 150 || got.Pulse != nil || len(got.Items) != 1 {
		t.Fatalf("updated = %+v", got)
	}

	if _, err := Insert(ctx, d, pid, uid, Input{MeasuredAt: now.AddDate(0, 0, -40), Items: []Item{{120, 80, nil}}}, now); err != nil {
		t.Fatal(err)
	}
	list, err := List(ctx, d, pid, now.AddDate(0, 0, -7), now.AddDate(0, 0, 1), 0)
	if err != nil || len(list) != 1 || list[0].ID != id || len(list[0].Items) != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}

	if err := Delete(ctx, d, id); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(ctx, d, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted err = %v", err)
	}
	var items int
	_ = d.QueryRow(`SELECT COUNT(*) FROM reading_items WHERE reading_id = ?`, id).Scan(&items)
	if items != 0 {
		t.Fatalf("items not cascaded: %d", items)
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func describe(b Bucket) map[string]any {
	return map[string]any{
		"date": b.Date, "period": b.Period, "sys": deref(b.Systolic), "dia": deref(b.Diastolic),
		"pulse": deref(b.Pulse), "count": b.Count,
	}
}
