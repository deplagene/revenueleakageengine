package nvidia

import (
	"fmt"

	documentdomain "github.com/deplagene/revenueleakageengine/internal/domain/document"
)

const PromptVersion = "document-intake-v1"

func extractionSystemPrompt() string {
	return `You are an extraction engine for Revenue Leakage Engine.
Return only valid JSON. Do not include markdown, comments, prose, or explanations.
Extract only facts explicitly present in the provided document text.
Do not infer missing prices, dates, currencies, IDs, quantities, or customer names.
If a value is absent, use null and add a validation issue.
Never calculate expected revenue, actual revenue, leakage, discounts, or root cause.
Never decide whether a leakage case exists.
Every extracted fact must include evidence_refs with page, row, field, quote, and source confidence when available.
All money amounts must use minor units and preserve currency.
Dates must be ISO-8601 strings when present.
The output schema is:
{
  "schema_version": "document_intake.v1",
  "draft_type": "contract_terms|usage_records|invoices|mixed_facts",
  "summary": "short factual summary",
  "contract_terms_draft": [],
  "usage_records_draft": [],
  "invoices_draft": [],
  "validation_issues": [],
  "requires_human_review": true
}`
}

func extractionUserPrompt(req promptRequest) string {
	return fmt.Sprintf(`Tenant ID: %s
Document ID: %s
File name: %s
Content type: %s
Source type: %s
Requested draft type: %s

Document text:
%s`,
		req.TenantID,
		req.DocumentID,
		req.FileName,
		req.ContentType,
		req.SourceType,
		req.DraftType,
		req.Text,
	)
}

type promptRequest struct {
	TenantID    string
	DocumentID  string
	FileName    string
	ContentType string
	SourceType  documentdomain.SourceType
	DraftType   documentdomain.DraftType
	Text        string
}
