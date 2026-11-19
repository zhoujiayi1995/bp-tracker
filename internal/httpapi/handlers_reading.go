package httpapi

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"bptracker/internal/auth"
	"bptracker/internal/authz"
	"bptracker/internal/db"
	"bptracker/internal/profile"
	"bptracker/internal/reading"
	"bptracker/internal/validate"
)

const (
	defaultRangeDays = 30
	maxRangeDays     = 366
	defaultListLimit = 200
	maxListLimit     = 1000
)

type itemDTO struct {
	Systolic  int  `json:"systolic"`
	Diastolic int  `json:"diastolic"`
	Pulse     *int `json:"pulse"`
}

type readingDTO struct {
	ID         int64     `json:"id"`
	ProfileID  int64     `json:"profileId"`
	MeasuredAt time.Time `json:"measuredAt"`
	Period     string    `json:"period"`
	Systolic   int       `json:"systolic"`
	Diastolic  int       `json:"diastolic"`
	Pulse      *int      `json:"pulse"`
	Note       string    `json:"note"`
	Items      []itemDTO `json:"items"`
}

type readingRequest struct {
	MeasuredAt *time.Time `json:"measuredAt"`
	Items      []itemDTO  `json:"items"`
	Note       string     `json:"note"`
}

func (req readingRequest) input() reading.Input {
	in := reading.Input{Note: req.Note}
	if req.MeasuredAt != nil {
		in.MeasuredAt = *req.MeasuredAt
	}
	for _, it := range req.Items {
		in.Items = append(in.Items, reading.Item{Systolic: it.Systolic, Diastolic: it.Diastolic, Pulse: it.Pulse})
	}
	return in
}

func (s *Server) toReadingDTO(r reading.Reading) readingDTO {
	dto := readingDTO{
		ID:         r.ID,
		ProfileID:  r.ProfileID,
		MeasuredAt: r.MeasuredAt.In(s.loc),
		Period:     string(reading.PeriodOf(r.MeasuredAt, s.loc)),
		Systolic:   r.Systolic,
		Diastolic:  r.Diastolic,
		Pulse:      r.Pulse,
		Note:       r.Note,
		Items:      make([]itemDTO, 0, len(r.Items)),
	}
	for _, it := range r.Items {
		dto.Items = append(dto.Items, itemDTO{Systolic: it.Systolic, Diastolic: it.Diastolic, Pulse: it.Pulse})
	}
	return dto
}

func (s *Server) createReading(w http.ResponseWriter, r *http.Request, u auth.User) error {
	p, err := s.profileForWrite(r, u)
	if err != nil {
		return err
	}
	var req readingRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	in := req.input()
	now := s.now()
	if err := in.Normalize(now); err != nil {
		return err
	}

	var id int64
	err = db.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var err error
		id, err = reading.Insert(r.Context(), tx, p.ID, u.ID, in, now)
		return err
	})
	if err != nil {
		return err
	}
	saved, err := reading.Get(r.Context(), s.db, id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, s.toReadingDTO(saved))
	return nil
}

func (s *Server) listReadings(w http.ResponseWriter, r *http.Request, u auth.User) error {
	p, err := s.profileForRead(r, u)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	var start, end time.Time
	if from, to := q.Get("from"), q.Get("to"); from != "" || to != "" {
		rng, err := reading.ParseDayRange(from, to, s.loc, s.now(), defaultRangeDays, maxRangeDays)
		if err != nil {
			return err
		}
		start, end = rng.Bounds()
	}
	limit := defaultListLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > maxListLimit {
			return validate.Field("limit", "range")
		}
		limit = n
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return validate.Field("offset", "range")
		}
		offset = n
	}
	list, err := reading.List(r.Context(), s.db, p.ID, start, end, limit, offset)
	if err != nil {
		return err
	}
	out := make([]readingDTO, 0, len(list))
	for _, rd := range list {
		out = append(out, s.toReadingDTO(rd))
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

type pointDTO struct {
	At        time.Time `json:"at"`
	Systolic  int       `json:"systolic"`
	Diastolic int       `json:"diastolic"`
	Pulse     *int      `json:"pulse"`
}

type bucketDTO struct {
	Date         string `json:"date"`
	Period       string `json:"period"`
	Systolic     *int   `json:"systolic"`
	Diastolic    *int   `json:"diastolic"`
	Pulse        *int   `json:"pulse"`
	Count        int    `json:"count"`
	SystolicSum  int    `json:"systolicSum"`
	DiastolicSum int    `json:"diastolicSum"`
	PulseSum     int    `json:"pulseSum"`
	PulseCount   int    `json:"pulseCount"`
}

type summaryDTO struct {
	Count     int  `json:"count"`
	Systolic  *int `json:"systolic"`
	Diastolic *int `json:"diastolic"`
	Pulse     *int `json:"pulse"`
}

type trendResponse struct {
	Mode    string      `json:"mode"`
	From    string      `json:"from"`
	To      string      `json:"to"`
	Points  []pointDTO  `json:"points,omitempty"`
	Buckets []bucketDTO `json:"buckets,omitempty"`
	Summary summaryDTO  `json:"summary"`
}

func (s *Server) trend(w http.ResponseWriter, r *http.Request, u auth.User) error {
	p, err := s.profileForRead(r, u)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	mode := q.Get("mode")
	if mode == "" {
		mode = "raw"
	}
	if mode != "raw" && mode != "ampm" {
		return validate.Field("mode", "unknown")
	}
	rng, err := reading.ParseDayRange(q.Get("from"), q.Get("to"), s.loc, s.now(), defaultRangeDays, maxRangeDays)
	if err != nil {
		return err
	}

	start, end := rng.Bounds()
	list, err := reading.List(r.Context(), s.db, p.ID, start, end, 0, 0)
	if err != nil {
		return err
	}

	sum := reading.Summarize(list)
	resp := trendResponse{
		Mode:    mode,
		From:    rng.From.Format("2006-01-02"),
		To:      rng.To.Format("2006-01-02"),
		Summary: summaryDTO{Count: sum.Count, Systolic: sum.Systolic, Diastolic: sum.Diastolic, Pulse: sum.Pulse},
	}
	if mode == "raw" {
		resp.Points = []pointDTO{}
		for _, pt := range reading.RawPoints(list) {
			resp.Points = append(resp.Points, pointDTO{At: pt.At.In(s.loc), Systolic: pt.Systolic, Diastolic: pt.Diastolic, Pulse: pt.Pulse})
		}
	} else {
		for _, b := range reading.PeriodBuckets(list, rng) {
			resp.Buckets = append(resp.Buckets, bucketDTO{
				Date: b.Date, Period: string(b.Period), Systolic: b.Systolic, Diastolic: b.Diastolic, Pulse: b.Pulse, Count: b.Count,
				SystolicSum: b.SystolicSum, DiastolicSum: b.DiastolicSum, PulseSum: b.PulseSum, PulseCount: b.PulseCount,
			})
		}
	}
	writeJSON(w, http.StatusOK, resp)
	return nil
}

func (s *Server) getReading(w http.ResponseWriter, r *http.Request, u auth.User) error {
	rd, _, err := s.readingWithAccess(r, u, authz.CanRead)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.toReadingDTO(rd))
	return nil
}

func (s *Server) updateReading(w http.ResponseWriter, r *http.Request, u auth.User) error {
	rd, _, err := s.readingWithAccess(r, u, authz.CanWrite)
	if err != nil {
		return err
	}
	var req readingRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	in := req.input()
	if in.MeasuredAt.IsZero() {
		in.MeasuredAt = rd.MeasuredAt
	}
	if err := in.Normalize(s.now()); err != nil {
		return err
	}
	err = db.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		return reading.Update(r.Context(), tx, rd.ID, in)
	})
	if err != nil {
		return err
	}
	saved, err := reading.Get(r.Context(), s.db, rd.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, s.toReadingDTO(saved))
	return nil
}

func (s *Server) deleteReading(w http.ResponseWriter, r *http.Request, u auth.User) error {
	rd, _, err := s.readingWithAccess(r, u, authz.CanWrite)
	if err != nil {
		return err
	}
	if err := reading.Delete(r.Context(), s.db, rd.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// readingWithAccess loads a reading and checks the caller's access through its profile.
func (s *Server) readingWithAccess(r *http.Request, u auth.User, allowed func(authz.Subject, authz.Target) bool) (reading.Reading, profile.Profile, error) {
	id, err := pathID(r)
	if err != nil {
		return reading.Reading{}, profile.Profile{}, err
	}
	rd, err := reading.Get(r.Context(), s.db, id)
	if err != nil {
		return reading.Reading{}, profile.Profile{}, err
	}
	p, err := s.loadProfile(r.Context(), u, rd.ProfileID, allowed)
	if err != nil {
		return reading.Reading{}, profile.Profile{}, err
	}
	return rd, p, nil
}
