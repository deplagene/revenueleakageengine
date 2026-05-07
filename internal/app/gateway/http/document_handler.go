package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	documentapp "github.com/deplagene/revenueleakageengine/internal/app/document"
	documentdomain "github.com/deplagene/revenueleakageengine/internal/domain/document"
	documentservice "github.com/deplagene/revenueleakageengine/internal/service/document"
	"github.com/go-chi/chi/v5"
)

const maxMultipartMemory = 16 << 20

type documentResponse struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	SourceType  string         `json:"source_type"`
	FileName    string         `json:"file_name"`
	ContentType string         `json:"content_type"`
	SizeBytes   int64          `json:"size_bytes"`
	SHA256Hash  string         `json:"sha256_hash"`
	StorageKey  string         `json:"storage_key"`
	Status      string         `json:"status"`
	UploadedAt  string         `json:"uploaded_at"`
	Metadata    map[string]any `json:"metadata"`
}

type draftResponse struct {
	ID                         string          `json:"id"`
	DocumentID                 string          `json:"document_id"`
	TenantID                   string          `json:"tenant_id"`
	Type                       string          `json:"type"`
	Status                     string          `json:"status"`
	Model                      string          `json:"model"`
	PromptVersion              string          `json:"prompt_version"`
	InputHash                  string          `json:"input_hash"`
	OutputJSON                 json.RawMessage `json:"output_json"`
	EvidenceJSON               json.RawMessage `json:"evidence_json"`
	ConfidenceScoreBasisPoints uint16          `json:"confidence_score_basis_points"`
	CreatedAt                  string          `json:"created_at"`
}

func (h *Handler) handleUploadDocuments(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse multipart form: %v", err))
		return
	}

	tenantID, err := parseUUID("tenant_id", r.FormValue("tenant_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sourceType := documentdomain.SourceType(strings.TrimSpace(r.FormValue("source_type")))
	if sourceType == "" {
		sourceType = documentdomain.SourceTypeMixed
	}

	headers := r.MultipartForm.File["documents"]
	if len(headers) == 0 {
		headers = r.MultipartForm.File["files"]
	}

	files := make([]documentapp.UploadFile, 0, len(headers))
	for _, header := range headers {
		file, err := header.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("open uploaded document: %v", err))
			return
		}
		content, readErr := io.ReadAll(io.LimitReader(file, documentapp.DefaultMaxFileBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("read uploaded document: %v", readErr))
			return
		}
		if closeErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("close uploaded document: %v", closeErr))
			return
		}

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = http.DetectContentType(content)
		}
		files = append(files, documentapp.UploadFile{
			FileName:    header.Filename,
			ContentType: contentType,
			Content:     content,
		})
	}

	result, err := h.documentCommands.UploadDocuments(r.Context(), documentapp.UploadDocumentsCommand{
		TenantID:   tenantID,
		SourceType: sourceType,
		Files:      files,
	})
	if err != nil {
		writeDocumentError(w, err)
		return
	}

	responses := make([]documentResponse, 0, len(result.Documents))
	for _, item := range result.Documents {
		responses = append(responses, newDocumentResponse(item))
	}
	writeJSON(w, http.StatusCreated, map[string]any{"documents": responses})
}

func (h *Handler) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseUUID("tenant_id", r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := optionalDocumentIntQuery("limit", r.URL.Query().Get("limit"), documentservice.DefaultListLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, err := optionalDocumentIntQuery("offset", r.URL.Query().Get("offset"), 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.documentQueries.ListDocuments(r.Context(), documentservice.ListDocumentsCommand{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeDocumentError(w, err)
		return
	}

	responses := make([]documentResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, newDocumentResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": responses})
}

func (h *Handler) handleExtractDocumentFacts(w http.ResponseWriter, r *http.Request) {
	documentID, err := parseUUID("document_id", chi.URLParam(r, "document_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		TenantID  string `json:"tenant_id"`
		DraftType string `json:"draft_type"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID, err := parseUUID("tenant_id", req.TenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	draftType := documentdomain.DraftType(strings.TrimSpace(req.DraftType))
	if draftType == "" {
		draftType = documentdomain.DraftTypeMixedFacts
	}

	result, err := h.documentCommands.ExtractDocumentFacts(r.Context(), documentapp.ExtractDocumentFactsCommand{
		TenantID:   tenantID,
		DocumentID: documentID,
		DraftType:  draftType,
	})
	if err != nil {
		writeDocumentError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, newDraftResponse(result.Draft))
}

func (h *Handler) handleListDocumentDrafts(w http.ResponseWriter, r *http.Request) {
	documentID, err := parseUUID("document_id", chi.URLParam(r, "document_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, err := parseUUID("tenant_id", r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.documentQueries.ListDrafts(r.Context(), documentservice.ListDraftsCommand{
		TenantID:   tenantID,
		DocumentID: documentID,
		Limit:      documentservice.DefaultListLimit,
	})
	if err != nil {
		writeDocumentError(w, err)
		return
	}

	responses := make([]draftResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, newDraftResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"drafts": responses})
}

func writeDocumentError(w http.ResponseWriter, err error) {
	switch {
	case strings.Contains(err.Error(), "not found"):
		writeError(w, http.StatusNotFound, err.Error())
	case strings.Contains(err.Error(), "required"),
		strings.Contains(err.Error(), "invalid"),
		strings.Contains(err.Error(), "too many"),
		strings.Contains(err.Error(), "too large"),
		strings.Contains(err.Error(), "unsupported"):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func newDocumentResponse(item documentdomain.Document) documentResponse {
	return documentResponse{
		ID:          item.ID.String(),
		TenantID:    item.TenantID.String(),
		SourceType:  string(item.SourceType),
		FileName:    item.FileName,
		ContentType: item.ContentType,
		SizeBytes:   item.SizeBytes,
		SHA256Hash:  item.SHA256Hash,
		StorageKey:  item.StorageKey,
		Status:      string(item.Status),
		UploadedAt:  item.UploadedAt.UTC().Format(time.RFC3339Nano),
		Metadata:    item.Metadata,
	}
}

func newDraftResponse(item documentdomain.ExtractionDraft) draftResponse {
	return draftResponse{
		ID:                         item.ID.String(),
		DocumentID:                 item.DocumentID.String(),
		TenantID:                   item.TenantID.String(),
		Type:                       string(item.Type),
		Status:                     string(item.Status),
		Model:                      item.Model,
		PromptVersion:              item.PromptVersion,
		InputHash:                  item.InputHash,
		OutputJSON:                 item.OutputJSON,
		EvidenceJSON:               item.EvidenceJSON,
		ConfidenceScoreBasisPoints: item.ConfidenceScore.BasisPoints,
		CreatedAt:                  item.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func optionalDocumentIntQuery(field, raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}
	return value, nil
}
