// Package reading holds blood pressure measurements: validation, averaging of repeated
// measurements, persistence and trend aggregation.
package reading

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"bptracker/internal/validate"
)

var ErrNotFound = errors.New("reading not found")

const (
	MaxItems = 3

	minSystolic, maxSystolic   = 60, 260
	minDiastolic, maxDiastolic = 30, 160
	minPulse, maxPulse         = 30, 200

	maxNoteRunes = 500
	// Allow small clock skew between the phone and the server.
	maxFutureSkew = 10 * time.Minute
)

var earliestMeasurement = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Item is one cuff measurement. A reading averages 1 to 3 of them.
type Item struct {
	Systolic  int
	Diastolic int
	Pulse     *int
}

type Reading struct {
	ID         int64
	ProfileID  int64
	MeasuredAt time.Time
	Systolic   int
	Diastolic  int
	Pulse      *int
	Note       string
	CreatedBy  int64
	Items      []Item
}

// Input is what a client submits to create or replace a reading.
type Input struct {
	MeasuredAt time.Time
	Items      []Item
	Note       string
}

// Normalize trims the note, defaults the time to now, and validates everything.
func (in *Input) Normalize(now time.Time) error {
	in.Note = strings.TrimSpace(in.Note)
	if in.MeasuredAt.IsZero() {
		in.MeasuredAt = now
	}
	if in.MeasuredAt.After(now.Add(maxFutureSkew)) {
		return validate.Field("measuredAt", "future")
	}
	if in.MeasuredAt.Before(earliestMeasurement) {
		return validate.Field("measuredAt", "too_old")
	}
	if utf8.RuneCountInString(in.Note) > maxNoteRunes {
		return validate.Field("note", "too_long")
	}
	return ValidateItems(in.Items)
}

func ValidateItems(items []Item) error {
	if len(items) == 0 || len(items) > MaxItems {
		return validate.Field("items", "count")
	}
	for i, it := range items {
		prefix := fmt.Sprintf("items[%d].", i)
		if it.Systolic < minSystolic || it.Systolic > maxSystolic {
			return validate.Field(prefix+"systolic", "range")
		}
		if it.Diastolic < minDiastolic || it.Diastolic > maxDiastolic {
			return validate.Field(prefix+"diastolic", "range")
		}
		if it.Systolic <= it.Diastolic {
			return validate.Field(prefix+"systolic", "not_above_diastolic")
		}
		if it.Pulse == nil {
			return validate.Field(prefix+"pulse", "required")
		}
		if *it.Pulse < minPulse || *it.Pulse > maxPulse {
			return validate.Field(prefix+"pulse", "range")
		}
	}
	return nil
}

// Average rounds half away from zero. Pulse is optional per item and is averaged over the
// items that have one; it is nil when none do.
func Average(items []Item) (systolic, diastolic int, pulse *int) {
	if len(items) == 0 {
		return 0, 0, nil
	}
	var sumSys, sumDia, sumPulse, pulseCount int
	for _, it := range items {
		sumSys += it.Systolic
		sumDia += it.Diastolic
		if it.Pulse != nil {
			sumPulse += *it.Pulse
			pulseCount++
		}
	}
	systolic = roundDiv(sumSys, len(items))
	diastolic = roundDiv(sumDia, len(items))
	if pulseCount > 0 {
		p := roundDiv(sumPulse, pulseCount)
		pulse = &p
	}
	return systolic, diastolic, pulse
}

func roundDiv(sum, n int) int {
	return int(math.Round(float64(sum) / float64(n)))
}
