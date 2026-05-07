// Package document contains immutable document intake entities and AI draft
// models. These models are boundary inputs, not core revenue facts.
package document

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

var (
	ErrTenantRequired        = errors.New("tenant id is required")
	ErrDocumentIDRequired    = errors.New("document id is required")
	ErrDocumentNameRequired  = errors.New("document file name is required")
	ErrContentTypeRequired   = errors.New("document content type is required")
	ErrDocumentSizeInvalid   = errors.New("document size must be positive")
	ErrDocumentHashRequired  = errors.New("document sha256 hash is required")
	ErrStorageKeyRequired    = errors.New("document storage key is required")
	ErrSourceTypeInvalid     = errors.New("document source type is invalid")
	ErrDocumentStatusInvalid = errors.New("document status is invalid")
	ErrDraftIDRequired       = errors.New("document extraction draft id is required")
	ErrDraftTypeInvalid      = errors.New("document extraction draft type is invalid")
	ErrDraftStatusInvalid    = errors.New("document extraction draft status is invalid")
	ErrModelRequired         = errors.New("document extraction model is required")
	ErrPromptVersionRequired = errors.New("document extraction prompt version is required")
	ErrInputHashRequired     = errors.New("document extraction input hash is required")
	ErrOutputJSONInvalid     = errors.New("document extraction output json is invalid")
	ErrEvidenceJSONInvalid   = errors.New("document extraction evidence json is invalid")
)

// SourceType describes the original business source of an uploaded document.
type SourceType string

const (
	SourceTypeContract      SourceType = "contract"
	SourceTypeInvoice       SourceType = "invoice"
	SourceTypeUsageExport   SourceType = "usage_export"
	SourceTypeBillingExport SourceType = "billing_export"
	SourceTypeMixed         SourceType = "mixed"
)

// Status describes the immutable document processing lifecycle.
type Status string

const (
	StatusUploaded  Status = "uploaded"
	StatusExtracted Status = "extracted"
	StatusFailed    Status = "failed"
)

// DraftType describes the expected shape of an extraction draft.
type DraftType string

const (
	DraftTypeContractTerms DraftType = "contract_terms"
	DraftTypeUsageRecords  DraftType = "usage_records"
	DraftTypeInvoices      DraftType = "invoices"
	DraftTypeMixedFacts    DraftType = "mixed_facts"
)

// DraftStatus describes human review state for extracted facts.
type DraftStatus string

const (
	DraftStatusPendingReview DraftStatus = "pending_review"
	DraftStatusApproved      DraftStatus = "approved"
	DraftStatusRejected      DraftStatus = "rejected"
)

// Document is the immutable uploaded source artifact metadata.
type Document struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	SourceType  SourceType
	FileName    string
	ContentType string
	SizeBytes   int64
	SHA256Hash  string
	StorageKey  string
	Status      Status
	UploadedAt  time.Time
	Metadata    map[string]any
}

// Validate checks document metadata before persistence.
func (d Document) Validate() error {
	if d.ID == uuid.Nil {
		return ErrDocumentIDRequired
	}
	if d.TenantID == uuid.Nil {
		return ErrTenantRequired
	}
	if strings.TrimSpace(d.FileName) == "" {
		return ErrDocumentNameRequired
	}
	if strings.TrimSpace(d.ContentType) == "" {
		return ErrContentTypeRequired
	}
	if d.SizeBytes <= 0 {
		return ErrDocumentSizeInvalid
	}
	if strings.TrimSpace(d.SHA256Hash) == "" {
		return ErrDocumentHashRequired
	}
	if strings.TrimSpace(d.StorageKey) == "" {
		return ErrStorageKeyRequired
	}
	if !d.SourceType.Valid() {
		return ErrSourceTypeInvalid
	}
	if !d.Status.Valid() {
		return ErrDocumentStatusInvalid
	}
	return nil
}

// ExtractionDraft is a structured AI extraction result awaiting human review.
type ExtractionDraft struct {
	ID              uuid.UUID
	DocumentID      uuid.UUID
	TenantID        uuid.UUID
	Type            DraftType
	Status          DraftStatus
	Model           string
	PromptVersion   string
	InputHash       string
	OutputJSON      json.RawMessage
	EvidenceJSON    json.RawMessage
	ConfidenceScore valueobject.ConfidenceScore
	CreatedAt       time.Time
	ReviewedBy      string
	ReviewedAt      *time.Time
	RejectionReason string
}

// Validate checks extraction output before persistence.
func (d ExtractionDraft) Validate() error {
	if d.ID == uuid.Nil {
		return ErrDraftIDRequired
	}
	if d.DocumentID == uuid.Nil {
		return ErrDocumentIDRequired
	}
	if d.TenantID == uuid.Nil {
		return ErrTenantRequired
	}
	if !d.Type.Valid() {
		return ErrDraftTypeInvalid
	}
	if !d.Status.Valid() {
		return ErrDraftStatusInvalid
	}
	if strings.TrimSpace(d.Model) == "" {
		return ErrModelRequired
	}
	if strings.TrimSpace(d.PromptVersion) == "" {
		return ErrPromptVersionRequired
	}
	if strings.TrimSpace(d.InputHash) == "" {
		return ErrInputHashRequired
	}
	if !json.Valid(d.OutputJSON) {
		return ErrOutputJSONInvalid
	}
	if len(d.EvidenceJSON) == 0 {
		d.EvidenceJSON = json.RawMessage("[]")
	}
	if !json.Valid(d.EvidenceJSON) {
		return ErrEvidenceJSONInvalid
	}
	return nil
}

// Valid reports whether the source type is supported by the intake pipeline.
func (s SourceType) Valid() bool {
	switch s {
	case SourceTypeContract, SourceTypeInvoice, SourceTypeUsageExport, SourceTypeBillingExport, SourceTypeMixed:
		return true
	default:
		return false
	}
}

// Valid reports whether the status is supported by the intake pipeline.
func (s Status) Valid() bool {
	switch s {
	case StatusUploaded, StatusExtracted, StatusFailed:
		return true
	default:
		return false
	}
}

// Valid reports whether the draft type is supported by the intake pipeline.
func (t DraftType) Valid() bool {
	switch t {
	case DraftTypeContractTerms, DraftTypeUsageRecords, DraftTypeInvoices, DraftTypeMixedFacts:
		return true
	default:
		return false
	}
}

// Valid reports whether the draft status is supported by the intake pipeline.
func (s DraftStatus) Valid() bool {
	switch s {
	case DraftStatusPendingReview, DraftStatusApproved, DraftStatusRejected:
		return true
	default:
		return false
	}
}
