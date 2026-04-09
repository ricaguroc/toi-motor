package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ricaguroc/toi-motor/internal/record"
)

// PostgresRecordStore implements record.RecordStore using PostgreSQL via pgx/v5.
type PostgresRecordStore struct {
	pool *pgxpool.Pool
}

// NewPostgresRecordStore returns a PostgresRecordStore backed by the given pool.
func NewPostgresRecordStore(pool *pgxpool.Pool) *PostgresRecordStore {
	return &PostgresRecordStore{pool: pool}
}

// Append inserts a Record into the records table.
// Returns an error if the record_id already exists.
func (s *PostgresRecordStore) Append(ctx context.Context, r record.Record) error {
	const q = `
		INSERT INTO records (
			id, record_id, occurred_at, ingested_at, source, record_type,
			entity_ref, actor_ref, title, payload, object_refs, checksum, tags, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13, $14
		)`

	objectRefs := pgtype.FlatArray[string](r.ObjectRefs)
	tags := pgtype.FlatArray[string](r.Tags)

	_, err := s.pool.Exec(ctx, q,
		r.ID,
		r.RecordID,
		r.OccurredAt,
		r.IngestedAt,
		r.Source,
		r.RecordType,
		r.EntityRef,
		r.ActorRef,
		r.Title,
		r.Payload,
		objectRefs,
		r.Checksum,
		tags,
		r.Metadata,
	)
	if err != nil {
		return fmt.Errorf("postgres: append record: %w", err)
	}

	return nil
}

// GetByID fetches a single Record by its record_id (business key).
// Returns record.ErrNotFound if no matching row exists.
func (s *PostgresRecordStore) GetByID(ctx context.Context, recordID uuid.UUID) (record.Record, error) {
	const q = `
		SELECT
			id, record_id, occurred_at, ingested_at, source, record_type,
			entity_ref, actor_ref, title, payload, object_refs, checksum, tags, metadata
		FROM records
		WHERE record_id = $1`

	row := s.pool.QueryRow(ctx, q, recordID)

	r, err := scanRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return record.Record{}, record.ErrNotFound
		}
		return record.Record{}, fmt.Errorf("postgres: get record by id: %w", err)
	}

	return r, nil
}

// List returns a paginated slice of records matching the given filter.
// Pagination is keyset-based on (occurred_at DESC, record_id DESC).
func (s *PostgresRecordStore) List(ctx context.Context, f record.Filter) (record.ListResult, error) {
	const q = `
		SELECT
			id, record_id, occurred_at, ingested_at, source, record_type,
			entity_ref, actor_ref, title, payload, object_refs, checksum, tags, metadata
		FROM records
		WHERE
			($1::text IS NULL OR entity_ref = $1)
			AND ($2::text IS NULL OR actor_ref = $2)
			AND ($3::text IS NULL OR record_type = $3)
			AND ($4::text IS NULL OR source = $4)
			AND ($5::text IS NULL OR $5 = ANY(tags))
			AND ($6::timestamptz IS NULL OR occurred_at >= $6)
			AND ($7::timestamptz IS NULL OR occurred_at <= $7)
			AND (
				$8::timestamptz IS NULL
				OR (occurred_at, record_id) < ($8, $9)
			)
		ORDER BY occurred_at DESC, record_id DESC
		LIMIT $10`

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	var (
		cursorTime *time.Time
		cursorID   *uuid.UUID
	)

	if f.Cursor != nil {
		cursorTime = &f.Cursor.OccurredAt
		cursorID = &f.Cursor.RecordID
	}

	rows, err := s.pool.Query(ctx, q,
		f.EntityRef,
		f.ActorRef,
		f.RecordType,
		f.Source,
		f.Tag,
		f.From,
		f.To,
		cursorTime,
		cursorID,
		limit+1, // fetch one extra to detect HasMore
	)
	if err != nil {
		return record.ListResult{}, fmt.Errorf("postgres: list records: %w", err)
	}
	defer rows.Close()

	var items []record.Record
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return record.ListResult{}, fmt.Errorf("postgres: scan record: %w", err)
		}
		items = append(items, r)
	}

	if err := rows.Err(); err != nil {
		return record.ListResult{}, fmt.Errorf("postgres: list rows error: %w", err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	result := record.ListResult{
		Items:   items,
		HasMore: hasMore,
	}

	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		next := &record.Cursor{
			OccurredAt: last.OccurredAt,
			RecordID:   last.RecordID,
		}
		result.Next = next
	}

	return result, nil
}

// cursorToken is the JSON shape stored inside the base64 cursor string.
// It is intentionally unexported — callers work with record.Cursor.
type cursorToken struct {
	OccurredAt time.Time `json:"occurred_at"`
	RecordID   uuid.UUID `json:"record_id"`
}

// EncodeCursor serialises a record.Cursor into a URL-safe base64 string.
func EncodeCursor(c record.Cursor) (string, error) {
	b, err := json.Marshal(cursorToken{OccurredAt: c.OccurredAt, RecordID: c.RecordID})
	if err != nil {
		return "", fmt.Errorf("postgres: encode cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// DecodeCursor deserialises a base64 cursor string into a record.Cursor.
func DecodeCursor(s string) (record.Cursor, error) {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return record.Cursor{}, fmt.Errorf("postgres: decode cursor (base64): %w", err)
	}
	var t cursorToken
	if err := json.Unmarshal(b, &t); err != nil {
		return record.Cursor{}, fmt.Errorf("postgres: decode cursor (json): %w", err)
	}
	return record.Cursor{OccurredAt: t.OccurredAt, RecordID: t.RecordID}, nil
}

// rowScanner abstracts pgx.Row and pgx.Rows so scanRecord works for both.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanRecord reads all columns in SELECT order into a record.Record.
func scanRecord(row rowScanner) (record.Record, error) {
	var r record.Record
	var objectRefs pgtype.FlatArray[string]
	var tags pgtype.FlatArray[string]

	err := row.Scan(
		&r.ID,
		&r.RecordID,
		&r.OccurredAt,
		&r.IngestedAt,
		&r.Source,
		&r.RecordType,
		&r.EntityRef,
		&r.ActorRef,
		&r.Title,
		&r.Payload,
		&objectRefs,
		&r.Checksum,
		&tags,
		&r.Metadata,
	)
	if err != nil {
		return record.Record{}, err
	}

	r.ObjectRefs = []string(objectRefs)
	r.Tags = []string(tags)

	return r, nil
}
