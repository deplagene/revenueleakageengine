// Package ai defines provider-neutral extraction contracts used by document
// intake application code.
package ai

import (
	"context"
	"encoding/json"

	documentdomain "github.com/deplagene/revenueleakageengine/internal/domain/document"
)

type ExtractRequest struct {
	TenantID    string
	DocumentID  string
	FileName    string
	ContentType string
	SourceType  documentdomain.SourceType
	DraftType   documentdomain.DraftType
	Content     []byte
}

type ExtractResult struct {
	Model           string
	PromptVersion   string
	OutputJSON      json.RawMessage
	EvidenceJSON    json.RawMessage
	ConfidenceBasis uint16
}

// Extractor turns one uploaded document into a structured draft JSON.
type Extractor interface {
	Extract(ctx context.Context, req ExtractRequest) (ExtractResult, error)
}
