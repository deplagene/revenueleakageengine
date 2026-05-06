// Package outbox persists application events until they are safely published to
// Kafka or another transport.
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	sqlitedb "github.com/deplagene/revenueleakageengine/internal/platform/sqlite/sqlc"
	"github.com/google/uuid"
)

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	StatusFailed    Status = "failed"

	defaultListLimit = 50
	maxListLimit     = 500
)

var (
	ErrStoreRequired    = errors.New("outbox store is required")
	ErrEventIDRequired  = errors.New("event id is required")
	ErrHandlerRequired  = errors.New("handler is required")
	ErrLimitInvalid     = errors.New("limit must be between 1 and 500")
	ErrRetryAtRequired  = errors.New("retry time is required")
	ErrPublishedAtEmpty = errors.New("published_at is required")
)

type Status string

// Record is a persisted outbound event ready for publishing.
type Record struct {
	ID           uuid.UUID
	Topic        string
	EventType    string
	TenantID     uuid.UUID
	ContractID   uuid.UUID
	PartitionKey string
	PayloadJSON  []byte
	Headers      map[string]string
	Status       Status
	Attempts     int64
	LastError    string
	AvailableAt  time.Time
	CreatedAt    time.Time
	PublishedAt  time.Time
}

type ListPendingCommand struct {
	AvailableAt time.Time
	Limit       int
}

func (c ListPendingCommand) Normalize() ListPendingCommand {
	if c.AvailableAt.IsZero() {
		c.AvailableAt = time.Now().UTC()
	}

	if c.Limit == 0 {
		c.Limit = defaultListLimit
	}

	return c
}

func (c ListPendingCommand) Validate() error {
	if c.Limit < 1 || c.Limit > maxListLimit {
		return ErrLimitInvalid
	}

	return nil
}

type InboxRecord struct {
	EventID     uuid.UUID
	Handler     string
	Topic       string
	SourceKey   string
	ProcessedAt time.Time
}

// SQLiteStore persists outbox and inbox rows through sqlc-generated queries.
type SQLiteStore struct {
	queries *sqlitedb.Queries
}

func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	if db == nil {
		return nil, ErrStoreRequired
	}

	return &SQLiteStore{
		queries: sqlitedb.New(db),
	}, nil
}

func (s *SQLiteStore) Append(ctx context.Context, envelope appevent.Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := envelope.Validate(); err != nil {
		return err
	}

	payload, err := envelope.MarshalJSONPayload()
	if err != nil {
		return fmt.Errorf("marshal envelope payload: %w", err)
	}

	headers, err := marshalHeaders(envelope)
	if err != nil {
		return err
	}

	if err := s.queries.CreateOutboxEvent(ctx, sqlitedb.CreateOutboxEventParams{
		ID:           envelope.EventID.String(),
		Topic:        envelope.Topic,
		EventType:    envelope.Type,
		TenantID:     envelope.TenantID.String(),
		ContractID:   nullableUUID(envelope.ContractID),
		PartitionKey: envelope.PartitionKey,
		PayloadJson:  string(payload),
		HeadersJson:  headers,
		Status:       string(StatusPending),
		AvailableAt:  formatStoredTime(envelope.OccurredAt),
		CreatedAt:    formatStoredTime(envelope.OccurredAt),
	}); err != nil {
		return fmt.Errorf("create outbox event: %w", err)
	}

	return nil
}

func (s *SQLiteStore) AppendBatch(ctx context.Context, envelopes []appevent.Envelope) error {
	for index, envelope := range envelopes {
		if err := s.Append(ctx, envelope); err != nil {
			return fmt.Errorf("append envelope[%d]: %w", index, err)
		}
	}

	return nil
}

func (s *SQLiteStore) ListPending(ctx context.Context, cmd ListPendingCommand) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cmd = cmd.Normalize()
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListPendingOutboxEvents(ctx, sqlitedb.ListPendingOutboxEventsParams{
		AvailableAt: formatStoredTime(cmd.AvailableAt),
		LimitCount:  int64(cmd.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list pending outbox events: %w", err)
	}

	records := make([]Record, 0, len(rows))
	for index, row := range rows {
		record, err := recordFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("outbox row[%d]: %w", index, err)
		}

		records = append(records, record)
	}

	return records, nil
}

func (s *SQLiteStore) MarkPublished(ctx context.Context, eventID uuid.UUID, publishedAt time.Time) error {
	if eventID == uuid.Nil {
		return ErrEventIDRequired
	}

	if publishedAt.IsZero() {
		return ErrPublishedAtEmpty
	}

	if err := s.queries.MarkOutboxEventPublished(ctx, sqlitedb.MarkOutboxEventPublishedParams{
		ID:          eventID.String(),
		PublishedAt: nullableStoredTime(publishedAt),
	}); err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}

	return nil
}

func (s *SQLiteStore) MarkFailed(ctx context.Context, eventID uuid.UUID, lastError string, retryAt time.Time) error {
	if eventID == uuid.Nil {
		return ErrEventIDRequired
	}

	if retryAt.IsZero() {
		return ErrRetryAtRequired
	}

	if err := s.queries.MarkOutboxEventFailed(ctx, sqlitedb.MarkOutboxEventFailedParams{
		ID:          eventID.String(),
		LastError:   strings.TrimSpace(lastError),
		AvailableAt: formatStoredTime(retryAt),
	}); err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}

	return nil
}

func (s *SQLiteStore) AlreadyProcessed(ctx context.Context, eventID uuid.UUID, handler string) (bool, error) {
	if eventID == uuid.Nil {
		return false, ErrEventIDRequired
	}

	handler = strings.TrimSpace(handler)
	if handler == "" {
		return false, ErrHandlerRequired
	}

	_, err := s.queries.GetInboxEvent(ctx, sqlitedb.GetInboxEventParams{
		EventID: eventID.String(),
		Handler: handler,
	})
	if err == nil {
		return true, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	return false, fmt.Errorf("get inbox event: %w", err)
}

func (s *SQLiteStore) MarkProcessed(ctx context.Context, record InboxRecord) error {
	if record.EventID == uuid.Nil {
		return ErrEventIDRequired
	}

	record.Handler = strings.TrimSpace(record.Handler)
	if record.Handler == "" {
		return ErrHandlerRequired
	}

	if record.ProcessedAt.IsZero() {
		record.ProcessedAt = time.Now().UTC()
	}

	if err := s.queries.CreateInboxEvent(ctx, sqlitedb.CreateInboxEventParams{
		EventID:     record.EventID.String(),
		Handler:     record.Handler,
		Topic:       strings.TrimSpace(record.Topic),
		SourceKey:   strings.TrimSpace(record.SourceKey),
		ProcessedAt: formatStoredTime(record.ProcessedAt),
	}); err != nil {
		return fmt.Errorf("create inbox event: %w", err)
	}

	return nil
}

func recordFromRow(row sqlitedb.OutboxEvent) (Record, error) {
	id, err := parseStoredUUID("outbox event id", row.ID)
	if err != nil {
		return Record{}, err
	}

	tenantID, err := parseStoredUUID("tenant id", row.TenantID)
	if err != nil {
		return Record{}, err
	}

	contractID, err := parseNullableStoredUUID("contract id", row.ContractID)
	if err != nil {
		return Record{}, err
	}

	headers, err := decodeHeaders(row.HeadersJson)
	if err != nil {
		return Record{}, err
	}

	availableAt, err := parseStoredTime("available_at", row.AvailableAt)
	if err != nil {
		return Record{}, err
	}

	createdAt, err := parseStoredTime("created_at", row.CreatedAt)
	if err != nil {
		return Record{}, err
	}

	publishedAt, err := parseNullableStoredTime("published_at", row.PublishedAt)
	if err != nil {
		return Record{}, err
	}

	return Record{
		ID:           id,
		Topic:        row.Topic,
		EventType:    row.EventType,
		TenantID:     tenantID,
		ContractID:   contractID,
		PartitionKey: row.PartitionKey,
		PayloadJSON:  []byte(row.PayloadJson),
		Headers:      headers,
		Status:       Status(row.Status),
		Attempts:     row.Attempts,
		LastError:    row.LastError,
		AvailableAt:  availableAt,
		CreatedAt:    createdAt,
		PublishedAt:  publishedAt,
	}, nil
}

func marshalHeaders(envelope appevent.Envelope) (string, error) {
	headers := map[string]string{
		"event_id":   envelope.EventID.String(),
		"event_type": envelope.Type,
		"tenant_id":  envelope.TenantID.String(),
	}

	if envelope.ContractID != uuid.Nil {
		headers["contract_id"] = envelope.ContractID.String()
	}

	if envelope.TraceID != "" {
		headers["trace_id"] = envelope.TraceID
	}

	encoded, err := json.Marshal(headers)
	if err != nil {
		return "", fmt.Errorf("marshal headers: %w", err)
	}

	return string(encoded), nil
}

func decodeHeaders(value string) (map[string]string, error) {
	if strings.TrimSpace(value) == "" {
		return map[string]string{}, nil
	}

	headers := make(map[string]string)
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return nil, fmt.Errorf("decode headers: %w", err)
	}

	return headers, nil
}

func parseStoredUUID(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse %s: %w", field, err)
	}

	return id, nil
}

func parseNullableStoredUUID(field string, value sql.NullString) (uuid.UUID, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return uuid.Nil, nil
	}

	return parseStoredUUID(field, value.String)
}

func parseStoredTime(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", field, err)
	}

	return parsed.UTC(), nil
}

func parseNullableStoredTime(field string, value sql.NullString) (time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return time.Time{}, nil
	}

	return parseStoredTime(field, value.String)
}

func formatStoredTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableStoredTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}

	return sql.NullString{
		String: formatStoredTime(value),
		Valid:  true,
	}
}

func nullableUUID(value uuid.UUID) sql.NullString {
	if value == uuid.Nil {
		return sql.NullString{}
	}

	return sql.NullString{
		String: value.String(),
		Valid:  true,
	}
}
