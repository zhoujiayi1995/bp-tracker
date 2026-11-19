package reading

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"bptracker/internal/db"
)

// Insert stores a validated input. Call it inside a transaction so the reading and its
// items are written atomically.
func Insert(ctx context.Context, q db.Querier, profileID, createdBy int64, in Input, now time.Time) (int64, error) {
	sys, dia, pulse := Average(in.Items)
	res, err := q.ExecContext(ctx, `
		INSERT INTO readings (profile_id, measured_at, systolic, diastolic, pulse, note, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		profileID, in.MeasuredAt.Unix(), sys, dia, nullable(pulse), in.Note, createdBy, now.Unix())
	if err != nil {
		return 0, fmt.Errorf("insert reading: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := insertItems(ctx, q, id, in.Items); err != nil {
		return 0, err
	}
	return id, nil
}

// Update replaces a reading's time, note and items. Call it inside a transaction.
func Update(ctx context.Context, q db.Querier, id int64, in Input) error {
	sys, dia, pulse := Average(in.Items)
	res, err := q.ExecContext(ctx, `
		UPDATE readings SET measured_at = ?, systolic = ?, diastolic = ?, pulse = ?, note = ?
		WHERE id = ?`,
		in.MeasuredAt.Unix(), sys, dia, nullable(pulse), in.Note, id)
	if err != nil {
		return fmt.Errorf("update reading: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM reading_items WHERE reading_id = ?`, id); err != nil {
		return fmt.Errorf("clear reading items: %w", err)
	}
	return insertItems(ctx, q, id, in.Items)
}

func Delete(ctx context.Context, q db.Querier, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM readings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete reading: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const selectReadings = `
	SELECT id, profile_id, measured_at, systolic, diastolic, pulse, note, created_by
	FROM readings`

func Get(ctx context.Context, q db.Querier, id int64) (Reading, error) {
	var r Reading
	var measuredAt int64
	var pulse sql.NullInt64
	err := q.QueryRowContext(ctx, selectReadings+` WHERE id = ?`, id).
		Scan(&r.ID, &r.ProfileID, &measuredAt, &r.Systolic, &r.Diastolic, &pulse, &r.Note, &r.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return Reading{}, ErrNotFound
	}
	if err != nil {
		return Reading{}, fmt.Errorf("get reading: %w", err)
	}
	r.MeasuredAt = time.Unix(measuredAt, 0)
	r.Pulse = intPtr(pulse)

	items, err := loadItems(ctx, q, `WHERE reading_id = ?`, id)
	if err != nil {
		return Reading{}, err
	}
	r.Items = items[r.ID]
	return r, nil
}

// List returns a page of readings with their items, newest first. Two zero times
// select all dates, and a non-positive limit selects all matching rows.
func List(ctx context.Context, q db.Querier, profileID int64, start, end time.Time, limit, offset int) ([]Reading, error) {
	if limit <= 0 {
		limit = -1 // SQLite: negative LIMIT means unbounded
	}
	filter := " WHERE profile_id = ?"
	args := []any{profileID}
	if !start.IsZero() || !end.IsZero() {
		filter += " AND measured_at >= ? AND measured_at < ?"
		args = append(args, start.Unix(), end.Unix())
	}
	args = append(args, limit, offset)

	rows, err := q.QueryContext(ctx, `
		WITH page AS (`+selectReadings+filter+`
			ORDER BY measured_at DESC, id DESC LIMIT ? OFFSET ?)
		SELECT p.id, p.profile_id, p.measured_at, p.systolic, p.diastolic, p.pulse, p.note, p.created_by,
		       i.systolic, i.diastolic, i.pulse
		FROM page p LEFT JOIN reading_items i ON i.reading_id = p.id
		ORDER BY p.measured_at DESC, p.id DESC, i.seq`, args...)
	if err != nil {
		return nil, fmt.Errorf("list readings: %w", err)
	}
	defer rows.Close()

	var out []Reading
	for rows.Next() {
		var r Reading
		var measuredAt int64
		var pulse, itemSys, itemDia, itemPulse sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ProfileID, &measuredAt, &r.Systolic, &r.Diastolic, &pulse, &r.Note, &r.CreatedBy,
			&itemSys, &itemDia, &itemPulse); err != nil {
			return nil, fmt.Errorf("scan reading: %w", err)
		}
		if len(out) == 0 || out[len(out)-1].ID != r.ID {
			r.MeasuredAt = time.Unix(measuredAt, 0)
			r.Pulse = intPtr(pulse)
			out = append(out, r)
		}
		if itemSys.Valid {
			last := &out[len(out)-1]
			last.Items = append(last.Items, Item{Systolic: int(itemSys.Int64), Diastolic: int(itemDia.Int64), Pulse: intPtr(itemPulse)})
		}
	}
	return out, rows.Err()
}

func insertItems(ctx context.Context, q db.Querier, readingID int64, items []Item) error {
	for i, it := range items {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO reading_items (reading_id, seq, systolic, diastolic, pulse) VALUES (?, ?, ?, ?, ?)`,
			readingID, i+1, it.Systolic, it.Diastolic, nullable(it.Pulse)); err != nil {
			return fmt.Errorf("insert reading item: %w", err)
		}
	}
	return nil
}

func loadItems(ctx context.Context, q db.Querier, where string, args ...any) (map[int64][]Item, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT reading_id, systolic, diastolic, pulse FROM reading_items `+where+` ORDER BY reading_id, seq`, args...)
	if err != nil {
		return nil, fmt.Errorf("load reading items: %w", err)
	}
	defer rows.Close()
	out := make(map[int64][]Item)
	for rows.Next() {
		var id int64
		var it Item
		var pulse sql.NullInt64
		if err := rows.Scan(&id, &it.Systolic, &it.Diastolic, &pulse); err != nil {
			return nil, fmt.Errorf("scan reading item: %w", err)
		}
		it.Pulse = intPtr(pulse)
		out[id] = append(out[id], it)
	}
	return out, rows.Err()
}

func nullable(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func intPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int64)
	return &i
}
