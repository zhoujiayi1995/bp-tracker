package reading

import (
	"sort"
	"time"

	"bptracker/internal/validate"
)

type Period string

const (
	AM      Period = "am"
	PM      Period = "pm"
	Evening Period = "evening"
)

// Periods is the day in order: morning, afternoon, evening.
var Periods = []Period{AM, PM, Evening}

const dateLayout = "2006-01-02"

// PeriodOf splits the local day into three periods: 00:00-11:59 morning,
// 12:00-17:59 afternoon, 18:00-23:59 evening.
func PeriodOf(t time.Time, loc *time.Location) Period {
	switch h := t.In(loc).Hour(); {
	case h < 12:
		return AM
	case h < 18:
		return PM
	default:
		return Evening
	}
}

// DayRange is an inclusive range of calendar days in a given location.
type DayRange struct {
	From time.Time // midnight of the first day
	To   time.Time // midnight of the last day
	loc  *time.Location
}

// Bounds returns the half-open instant interval [start, end) covered by the range.
func (r DayRange) Bounds() (start, end time.Time) {
	return r.From, addDays(r.To, 1)
}

func (r DayRange) Days() int {
	n := 0
	for d := r.From; !d.After(r.To); d = addDays(d, 1) {
		n++
	}
	return n
}

// ParseDayRange reads YYYY-MM-DD dates (both optional). Missing "to" means today; missing
// "from" means defaultDays ending on "to". Ranges longer than maxDays are rejected.
func ParseDayRange(fromStr, toStr string, loc *time.Location, now time.Time, defaultDays, maxDays int) (DayRange, error) {
	to := midnight(now.In(loc))
	if toStr != "" {
		t, err := time.ParseInLocation(dateLayout, toStr, loc)
		if err != nil {
			return DayRange{}, validate.Field("to", "format")
		}
		to = t
	}
	from := addDays(to, -(defaultDays - 1))
	if fromStr != "" {
		f, err := time.ParseInLocation(dateLayout, fromStr, loc)
		if err != nil {
			return DayRange{}, validate.Field("from", "format")
		}
		from = f
	}
	if from.After(to) {
		return DayRange{}, validate.Field("from", "after_to")
	}
	r := DayRange{From: from, To: to, loc: loc}
	if r.Days() > maxDays {
		return DayRange{}, validate.Field("from", "range_too_long")
	}
	return r, nil
}

type Point struct {
	At        time.Time
	Systolic  int
	Diastolic int
	Pulse     *int
}

// RawPoints returns one point per reading in chronological order.
func RawPoints(readings []Reading) []Point {
	out := make([]Point, 0, len(readings))
	for _, r := range readings {
		out = append(out, Point{At: r.MeasuredAt, Systolic: r.Systolic, Diastolic: r.Diastolic, Pulse: r.Pulse})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// Bucket is the average of every reading in one period of a day. Values are nil when
// that period has no readings, so charts can show a gap instead of a misleading line.
type Bucket struct {
	Date         string
	Period       Period
	Systolic     *int
	Diastolic    *int
	Pulse        *int
	Count        int
	SystolicSum  int
	DiastolicSum int
	PulseSum     int
	PulseCount   int
}

// PeriodBuckets emits one bucket per period (morning, afternoon, evening) for every
// day in the range, in order.
func PeriodBuckets(readings []Reading, r DayRange) []Bucket {
	type acc struct{ sys, dia, pulse, n, pulseN int }
	sums := make(map[string]*acc)
	key := func(date string, p Period) string { return date + "|" + string(p) }

	for _, rd := range readings {
		local := rd.MeasuredAt.In(r.loc)
		k := key(local.Format(dateLayout), PeriodOf(local, r.loc))
		a := sums[k]
		if a == nil {
			a = &acc{}
			sums[k] = a
		}
		a.sys += rd.Systolic
		a.dia += rd.Diastolic
		a.n++
		if rd.Pulse != nil {
			a.pulse += *rd.Pulse
			a.pulseN++
		}
	}

	out := make([]Bucket, 0, r.Days()*len(Periods))
	for d := r.From; !d.After(r.To); d = addDays(d, 1) {
		date := d.Format(dateLayout)
		for _, p := range Periods {
			b := Bucket{Date: date, Period: p}
			if a := sums[key(date, p)]; a != nil {
				sys, dia := roundDiv(a.sys, a.n), roundDiv(a.dia, a.n)
				b.Systolic, b.Diastolic, b.Count = &sys, &dia, a.n
				b.SystolicSum, b.DiastolicSum = a.sys, a.dia
				b.PulseSum, b.PulseCount = a.pulse, a.pulseN
				if a.pulseN > 0 {
					pulse := roundDiv(a.pulse, a.pulseN)
					b.Pulse = &pulse
				}
			}
			out = append(out, b)
		}
	}
	return out
}

type Summary struct {
	Count     int
	Systolic  *int
	Diastolic *int
	Pulse     *int
}

func Summarize(readings []Reading) Summary {
	s := Summary{Count: len(readings)}
	if len(readings) == 0 {
		return s
	}
	var sys, dia, pulse, pulseN int
	for _, r := range readings {
		sys += r.Systolic
		dia += r.Diastolic
		if r.Pulse != nil {
			pulse += *r.Pulse
			pulseN++
		}
	}
	avgSys, avgDia := roundDiv(sys, len(readings)), roundDiv(dia, len(readings))
	s.Systolic, s.Diastolic = &avgSys, &avgDia
	if pulseN > 0 {
		p := roundDiv(pulse, pulseN)
		s.Pulse = &p
	}
	return s
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// addDays goes through the calendar instead of adding 24h, so DST changes cannot shift days.
func addDays(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+n, 0, 0, 0, 0, t.Location())
}
