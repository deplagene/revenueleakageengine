package document

import (
	"context"
	"errors"
	"fmt"

	documentdomain "github.com/deplagene/revenueleakageengine/internal/domain/document"
	"github.com/google/uuid"
)

var (
	ErrTenantRequired   = documentdomain.ErrTenantRequired
	ErrDocumentRequired = documentdomain.ErrDocumentIDRequired
)

// Service applies document intake validation before persistence.
type Service struct {
	store Store
}

// NewService creates a document intake service.
func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, ErrStoreRequired
	}
	return &Service{store: store}, nil
}

func (s *Service) UpsertDocument(ctx context.Context, doc documentdomain.Document) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	if err := s.store.UpsertDocument(ctx, doc); err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}
	return nil
}

func (s *Service) GetDocument(ctx context.Context, tenantID, documentID uuid.UUID) (documentdomain.Document, error) {
	if tenantID == uuid.Nil {
		return documentdomain.Document{}, ErrTenantRequired
	}
	if documentID == uuid.Nil {
		return documentdomain.Document{}, ErrDocumentRequired
	}

	doc, err := s.store.GetDocument(ctx, tenantID, documentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return documentdomain.Document{}, ErrNotFound
		}
		return documentdomain.Document{}, fmt.Errorf("get document: %w", err)
	}
	return doc, nil
}

func (s *Service) ListDocuments(ctx context.Context, cmd ListDocumentsCommand) ([]documentdomain.Document, error) {
	cmd = cmd.Normalize()
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	items, err := s.store.ListDocuments(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	return items, nil
}

func (s *Service) CreateDraft(ctx context.Context, draft documentdomain.ExtractionDraft) error {
	if err := draft.Validate(); err != nil {
		return err
	}
	if err := s.store.CreateDraft(ctx, draft); err != nil {
		return fmt.Errorf("create document extraction draft: %w", err)
	}
	return nil
}

func (s *Service) GetDraft(ctx context.Context, tenantID, draftID uuid.UUID) (documentdomain.ExtractionDraft, error) {
	if tenantID == uuid.Nil {
		return documentdomain.ExtractionDraft{}, ErrTenantRequired
	}
	if draftID == uuid.Nil {
		return documentdomain.ExtractionDraft{}, documentdomain.ErrDraftIDRequired
	}

	draft, err := s.store.GetDraft(ctx, tenantID, draftID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return documentdomain.ExtractionDraft{}, ErrNotFound
		}
		return documentdomain.ExtractionDraft{}, fmt.Errorf("get document extraction draft: %w", err)
	}
	return draft, nil
}

func (s *Service) ListDrafts(ctx context.Context, cmd ListDraftsCommand) ([]documentdomain.ExtractionDraft, error) {
	cmd = cmd.Normalize()
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	items, err := s.store.ListDrafts(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("list document extraction drafts: %w", err)
	}
	return items, nil
}
