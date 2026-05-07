package document

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	documentdomain "github.com/deplagene/revenueleakageengine/internal/domain/document"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	sqlitedb "github.com/deplagene/revenueleakageengine/internal/platform/sqlite/sqlc"
	"github.com/google/uuid"
)

var _ Store = (*SQLiteStore)(nil)

// SQLiteStore persists document intake metadata and drafts in SQLite.
type SQLiteStore struct {
	queries *sqlitedb.Queries
}

// NewSQLiteStore creates a document store backed by SQLite sqlc queries.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{queries: sqlitedb.New(db)}
}

func (s *SQLiteStore) UpsertDocument(ctx context.Context, doc documentdomain.Document) error {
	metadataJSON, err := encodeJSONMap("document metadata", doc.Metadata)
	if err != nil {
		return err
	}

	if err := s.queries.CreateDocument(ctx, sqlitedb.CreateDocumentParams{
		ID:           doc.ID.String(),
		TenantID:     doc.TenantID.String(),
		SourceType:   string(doc.SourceType),
		FileName:     doc.FileName,
		ContentType:  doc.ContentType,
		SizeBytes:    doc.SizeBytes,
		Sha256Hash:   doc.SHA256Hash,
		StorageKey:   doc.StorageKey,
		Status:       string(doc.Status),
		UploadedAt:   formatStoredTime(doc.UploadedAt),
		MetadataJson: metadataJSON,
	}); err != nil {
		return fmt.Errorf("create document: %w", err)
	}

	return nil
}

func (s *SQLiteStore) GetDocument(ctx context.Context, tenantID, documentID uuid.UUID) (documentdomain.Document, error) {
	row, err := s.queries.GetDocument(ctx, sqlitedb.GetDocumentParams{
		TenantID: tenantID.String(),
		ID:       documentID.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return documentdomain.Document{}, ErrNotFound
		}
		return documentdomain.Document{}, fmt.Errorf("get document row: %w", err)
	}

	doc, err := documentFromRow(row)
	if err != nil {
		return documentdomain.Document{}, err
	}
	return doc, nil
}

func (s *SQLiteStore) ListDocuments(ctx context.Context, cmd ListDocumentsCommand) ([]documentdomain.Document, error) {
	rows, err := s.queries.ListDocuments(ctx, sqlitedb.ListDocumentsParams{
		TenantID: cmd.TenantID.String(),
		Limit:    int64(cmd.Limit),
		Offset:   int64(cmd.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list document rows: %w", err)
	}

	items := make([]documentdomain.Document, 0, len(rows))
	for index, row := range rows {
		item, err := documentFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("document row[%d]: %w", index, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *SQLiteStore) CreateDraft(ctx context.Context, draft documentdomain.ExtractionDraft) error {
	if err := s.queries.CreateDocumentExtractionDraft(ctx, sqlitedb.CreateDocumentExtractionDraftParams{
		ID:                         draft.ID.String(),
		DocumentID:                 draft.DocumentID.String(),
		TenantID:                   draft.TenantID.String(),
		DraftType:                  string(draft.Type),
		Status:                     string(draft.Status),
		Model:                      draft.Model,
		PromptVersion:              draft.PromptVersion,
		InputHash:                  draft.InputHash,
		OutputJson:                 string(draft.OutputJSON),
		EvidenceJson:               string(draft.EvidenceJSON),
		ConfidenceScoreBasisPoints: int64(draft.ConfidenceScore.BasisPoints),
		CreatedAt:                  formatStoredTime(draft.CreatedAt),
		ReviewedBy:                 draft.ReviewedBy,
		ReviewedAt:                 nullableStoredTime(draft.ReviewedAt),
		RejectionReason:            draft.RejectionReason,
	}); err != nil {
		return fmt.Errorf("create document extraction draft: %w", err)
	}

	return nil
}

func (s *SQLiteStore) GetDraft(ctx context.Context, tenantID, draftID uuid.UUID) (documentdomain.ExtractionDraft, error) {
	row, err := s.queries.GetDocumentExtractionDraft(ctx, sqlitedb.GetDocumentExtractionDraftParams{
		TenantID: tenantID.String(),
		ID:       draftID.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return documentdomain.ExtractionDraft{}, ErrNotFound
		}
		return documentdomain.ExtractionDraft{}, fmt.Errorf("get document extraction draft row: %w", err)
	}

	draft, err := draftFromRow(row)
	if err != nil {
		return documentdomain.ExtractionDraft{}, err
	}
	return draft, nil
}

func (s *SQLiteStore) ListDrafts(ctx context.Context, cmd ListDraftsCommand) ([]documentdomain.ExtractionDraft, error) {
	rows, err := s.queries.ListDocumentExtractionDrafts(ctx, sqlitedb.ListDocumentExtractionDraftsParams{
		TenantID:   cmd.TenantID.String(),
		DocumentID: cmd.DocumentID.String(),
		Limit:      int64(cmd.Limit),
		Offset:     int64(cmd.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list document extraction draft rows: %w", err)
	}

	items := make([]documentdomain.ExtractionDraft, 0, len(rows))
	for index, row := range rows {
		item, err := draftFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("draft row[%d]: %w", index, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func documentFromRow(row sqlitedb.Document) (documentdomain.Document, error) {
	id, err := parseStoredUUID("document id", row.ID)
	if err != nil {
		return documentdomain.Document{}, err
	}
	tenantID, err := parseStoredUUID("document tenant id", row.TenantID)
	if err != nil {
		return documentdomain.Document{}, err
	}
	uploadedAt, err := parseStoredTime("document uploaded at", row.UploadedAt)
	if err != nil {
		return documentdomain.Document{}, err
	}
	metadata, err := decodeJSONMap("document metadata", row.MetadataJson)
	if err != nil {
		return documentdomain.Document{}, err
	}

	return documentdomain.Document{
		ID:          id,
		TenantID:    tenantID,
		SourceType:  documentdomain.SourceType(row.SourceType),
		FileName:    row.FileName,
		ContentType: row.ContentType,
		SizeBytes:   row.SizeBytes,
		SHA256Hash:  row.Sha256Hash,
		StorageKey:  row.StorageKey,
		Status:      documentdomain.Status(row.Status),
		UploadedAt:  uploadedAt,
		Metadata:    metadata,
	}, nil
}

func draftFromRow(row sqlitedb.DocumentExtractionDraft) (documentdomain.ExtractionDraft, error) {
	id, err := parseStoredUUID("document extraction draft id", row.ID)
	if err != nil {
		return documentdomain.ExtractionDraft{}, err
	}
	documentID, err := parseStoredUUID("document id", row.DocumentID)
	if err != nil {
		return documentdomain.ExtractionDraft{}, err
	}
	tenantID, err := parseStoredUUID("document extraction tenant id", row.TenantID)
	if err != nil {
		return documentdomain.ExtractionDraft{}, err
	}
	createdAt, err := parseStoredTime("document extraction created at", row.CreatedAt)
	if err != nil {
		return documentdomain.ExtractionDraft{}, err
	}
	reviewedAt, err := parseNullableStoredTime("document extraction reviewed at", row.ReviewedAt)
	if err != nil {
		return documentdomain.ExtractionDraft{}, err
	}
	if row.ConfidenceScoreBasisPoints < 0 ||
		row.ConfidenceScoreBasisPoints > int64(valueobject.MaxConfidenceBasisPoints) {
		return documentdomain.ExtractionDraft{}, fmt.Errorf(
			"build confidence score: basis points out of range: %d",
			row.ConfidenceScoreBasisPoints,
		)
	}
	confidence, err := valueobject.NewConfidenceScore(uint16(row.ConfidenceScoreBasisPoints))
	if err != nil {
		return documentdomain.ExtractionDraft{}, fmt.Errorf("build confidence score: %w", err)
	}

	return documentdomain.ExtractionDraft{
		ID:              id,
		DocumentID:      documentID,
		TenantID:        tenantID,
		Type:            documentdomain.DraftType(row.DraftType),
		Status:          documentdomain.DraftStatus(row.Status),
		Model:           row.Model,
		PromptVersion:   row.PromptVersion,
		InputHash:       row.InputHash,
		OutputJSON:      json.RawMessage(row.OutputJson),
		EvidenceJSON:    json.RawMessage(row.EvidenceJson),
		ConfidenceScore: confidence,
		CreatedAt:       createdAt,
		ReviewedBy:      row.ReviewedBy,
		ReviewedAt:      reviewedAt,
		RejectionReason: row.RejectionReason,
	}, nil
}

func encodeJSONMap(field string, value map[string]any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode %s: %w", field, err)
	}
	return string(encoded), nil
}

func decodeJSONMap(field, raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("decode %s: %w", field, err)
	}
	return result, nil
}

func parseStoredUUID(field, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse %s: %w", field, err)
	}
	return id, nil
}

func formatStoredTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseStoredTime(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", field, err)
	}
	return parsed, nil
}

func nullableStoredTime(value *time.Time) sql.NullString {
	if value == nil || value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{
		String: formatStoredTime(*value),
		Valid:  true,
	}
}

func parseNullableStoredTime(field string, value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseStoredTime(field, value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
