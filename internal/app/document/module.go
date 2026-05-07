package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	documentdomain "github.com/deplagene/revenueleakageengine/internal/domain/document"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	platformai "github.com/deplagene/revenueleakageengine/internal/platform/ai"
	"github.com/deplagene/revenueleakageengine/internal/platform/storage"
	documentservice "github.com/deplagene/revenueleakageengine/internal/service/document"
	"github.com/google/uuid"
)

const (
	DefaultMaxFilesPerUpload = 5
	DefaultMaxFileBytes      = 10 << 20
)

var (
	ErrServiceRequired         = errors.New("document service is required")
	ErrStorageRequired         = errors.New("document storage is required")
	ErrExtractorRequired       = errors.New("document extractor is required")
	ErrNoDocumentsUploaded     = errors.New("no documents uploaded")
	ErrTooManyDocuments        = errors.New("too many documents in one upload")
	ErrDocumentTooLarge        = errors.New("document file is too large")
	ErrDocumentContentEmpty    = errors.New("document content is empty")
	ErrDocumentFileNameInvalid = errors.New("document file name is invalid")
)

type Config struct {
	MaxFilesPerUpload int
	MaxFileBytes      int64
}

type Commands struct {
	service   *documentservice.Service
	storage   storage.Store
	extractor platformai.Extractor
	cfg       Config
	now       func() time.Time
}

func NewCommands(
	service *documentservice.Service,
	storage storage.Store,
	extractor platformai.Extractor,
	cfg Config,
) (*Commands, error) {
	if service == nil {
		return nil, ErrServiceRequired
	}
	if storage == nil {
		return nil, ErrStorageRequired
	}
	if extractor == nil {
		return nil, ErrExtractorRequired
	}
	if cfg.MaxFilesPerUpload == 0 {
		cfg.MaxFilesPerUpload = DefaultMaxFilesPerUpload
	}
	if cfg.MaxFileBytes == 0 {
		cfg.MaxFileBytes = DefaultMaxFileBytes
	}
	return &Commands{
		service:   service,
		storage:   storage,
		extractor: extractor,
		cfg:       cfg,
		now:       time.Now,
	}, nil
}

type UploadFile struct {
	FileName    string
	ContentType string
	Content     []byte
	Metadata    map[string]any
}

type UploadDocumentsCommand struct {
	TenantID   uuid.UUID
	SourceType documentdomain.SourceType
	Files      []UploadFile
}

type UploadDocumentsResult struct {
	Documents []documentdomain.Document
}

func (c *Commands) UploadDocuments(ctx context.Context, cmd UploadDocumentsCommand) (UploadDocumentsResult, error) {
	if cmd.TenantID == uuid.Nil {
		return UploadDocumentsResult{}, documentdomain.ErrTenantRequired
	}
	if !cmd.SourceType.Valid() {
		return UploadDocumentsResult{}, documentdomain.ErrSourceTypeInvalid
	}
	if len(cmd.Files) == 0 {
		return UploadDocumentsResult{}, ErrNoDocumentsUploaded
	}
	if len(cmd.Files) > c.cfg.MaxFilesPerUpload {
		return UploadDocumentsResult{}, fmt.Errorf("%w: max %d", ErrTooManyDocuments, c.cfg.MaxFilesPerUpload)
	}

	docs := make([]documentdomain.Document, 0, len(cmd.Files))
	for _, file := range cmd.Files {
		doc, err := c.uploadOne(ctx, cmd.TenantID, cmd.SourceType, file)
		if err != nil {
			return UploadDocumentsResult{}, err
		}
		docs = append(docs, doc)
	}

	return UploadDocumentsResult{Documents: docs}, nil
}

type ExtractDocumentFactsCommand struct {
	TenantID   uuid.UUID
	DocumentID uuid.UUID
	DraftType  documentdomain.DraftType
}

type ExtractDocumentFactsResult struct {
	Draft documentdomain.ExtractionDraft
}

func (c *Commands) ExtractDocumentFacts(
	ctx context.Context,
	cmd ExtractDocumentFactsCommand,
) (ExtractDocumentFactsResult, error) {
	if cmd.TenantID == uuid.Nil {
		return ExtractDocumentFactsResult{}, documentdomain.ErrTenantRequired
	}
	if cmd.DocumentID == uuid.Nil {
		return ExtractDocumentFactsResult{}, documentdomain.ErrDocumentIDRequired
	}
	if !cmd.DraftType.Valid() {
		return ExtractDocumentFactsResult{}, documentdomain.ErrDraftTypeInvalid
	}

	doc, err := c.service.GetDocument(ctx, cmd.TenantID, cmd.DocumentID)
	if err != nil {
		return ExtractDocumentFactsResult{}, err
	}
	object, err := c.storage.Get(ctx, doc.StorageKey)
	if err != nil {
		return ExtractDocumentFactsResult{}, fmt.Errorf("read source document: %w", err)
	}

	extracted, err := c.extractor.Extract(ctx, platformai.ExtractRequest{
		TenantID:    cmd.TenantID.String(),
		DocumentID:  cmd.DocumentID.String(),
		FileName:    doc.FileName,
		ContentType: doc.ContentType,
		SourceType:  doc.SourceType,
		DraftType:   cmd.DraftType,
		Content:     object.Content,
	})
	if err != nil {
		return ExtractDocumentFactsResult{}, fmt.Errorf("extract document facts: %w", err)
	}

	confidence, err := valueobject.NewConfidenceScore(extracted.ConfidenceBasis)
	if err != nil {
		return ExtractDocumentFactsResult{}, fmt.Errorf("build extraction confidence: %w", err)
	}
	inputHash := sha256.Sum256(object.Content)
	draft := documentdomain.ExtractionDraft{
		ID:              uuid.New(),
		DocumentID:      doc.ID,
		TenantID:        doc.TenantID,
		Type:            cmd.DraftType,
		Status:          documentdomain.DraftStatusPendingReview,
		Model:           extracted.Model,
		PromptVersion:   extracted.PromptVersion,
		InputHash:       hex.EncodeToString(inputHash[:]),
		OutputJSON:      extracted.OutputJSON,
		EvidenceJSON:    extracted.EvidenceJSON,
		ConfidenceScore: confidence,
		CreatedAt:       c.now(),
	}
	if err := c.service.CreateDraft(ctx, draft); err != nil {
		return ExtractDocumentFactsResult{}, err
	}

	return ExtractDocumentFactsResult{Draft: draft}, nil
}

type Queries struct {
	service *documentservice.Service
}

func NewQueries(service *documentservice.Service) (*Queries, error) {
	if service == nil {
		return nil, ErrServiceRequired
	}
	return &Queries{service: service}, nil
}

func (q *Queries) ListDocuments(
	ctx context.Context,
	cmd documentservice.ListDocumentsCommand,
) ([]documentdomain.Document, error) {
	return q.service.ListDocuments(ctx, cmd)
}

func (q *Queries) GetDocument(ctx context.Context, tenantID, documentID uuid.UUID) (documentdomain.Document, error) {
	return q.service.GetDocument(ctx, tenantID, documentID)
}

func (q *Queries) ListDrafts(
	ctx context.Context,
	cmd documentservice.ListDraftsCommand,
) ([]documentdomain.ExtractionDraft, error) {
	return q.service.ListDrafts(ctx, cmd)
}

func (c *Commands) uploadOne(
	ctx context.Context,
	tenantID uuid.UUID,
	sourceType documentdomain.SourceType,
	file UploadFile,
) (documentdomain.Document, error) {
	if strings.TrimSpace(file.FileName) == "" {
		return documentdomain.Document{}, ErrDocumentFileNameInvalid
	}
	if len(file.Content) == 0 {
		return documentdomain.Document{}, ErrDocumentContentEmpty
	}
	if int64(len(file.Content)) > c.cfg.MaxFileBytes {
		return documentdomain.Document{}, fmt.Errorf("%w: max %d bytes", ErrDocumentTooLarge, c.cfg.MaxFileBytes)
	}

	documentID := uuid.New()
	hash := sha256.Sum256(file.Content)
	hashHex := hex.EncodeToString(hash[:])
	fileName := safeFileName(file.FileName)
	storageKey := path.Join(tenantID.String(), documentID.String(), fileName)
	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := c.storage.Put(ctx, storage.Object{
		Key:         storageKey,
		Content:     file.Content,
		ContentType: contentType,
	}); err != nil {
		return documentdomain.Document{}, fmt.Errorf("store source document: %w", err)
	}

	doc := documentdomain.Document{
		ID:          documentID,
		TenantID:    tenantID,
		SourceType:  sourceType,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   int64(len(file.Content)),
		SHA256Hash:  hashHex,
		StorageKey:  storageKey,
		Status:      documentdomain.StatusUploaded,
		UploadedAt:  c.now(),
		Metadata:    file.Metadata,
	}
	if err := c.service.UpsertDocument(ctx, doc); err != nil {
		return documentdomain.Document{}, err
	}
	return doc, nil
}

var unsafeFileNameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeFileName(value string) string {
	base := path.Base(strings.TrimSpace(value))
	base = unsafeFileNameChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, ".-")
	if base == "" {
		return "document"
	}
	return base
}
