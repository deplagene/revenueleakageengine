package document

import (
	"context"

	documentdomain "github.com/deplagene/revenueleakageengine/internal/domain/document"
	"github.com/google/uuid"
)

// Store defines document intake persistence from the service's needs.
type Store interface {
	UpsertDocument(ctx context.Context, document documentdomain.Document) error
	GetDocument(ctx context.Context, tenantID, documentID uuid.UUID) (documentdomain.Document, error)
	ListDocuments(ctx context.Context, cmd ListDocumentsCommand) ([]documentdomain.Document, error)
	CreateDraft(ctx context.Context, draft documentdomain.ExtractionDraft) error
	GetDraft(ctx context.Context, tenantID, draftID uuid.UUID) (documentdomain.ExtractionDraft, error)
	ListDrafts(ctx context.Context, cmd ListDraftsCommand) ([]documentdomain.ExtractionDraft, error)
}
