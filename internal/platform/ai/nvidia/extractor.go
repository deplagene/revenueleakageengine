// Package nvidia provides an NVIDIA API Catalog backed document extractor.
package nvidia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	platformai "github.com/deplagene/revenueleakageengine/internal/platform/ai"
)

const (
	DefaultBaseURL = "https://integrate.api.nvidia.com/v1"
	DefaultModel   = "nvidia/llama-3.1-nemotron-nano-8b-v1"
)

var (
	ErrAPIKeyRequired       = errors.New("nvidia api key is required")
	ErrUnsupportedContent   = errors.New("document content type is not supported by text extractor")
	ErrEmptyModelResponse   = errors.New("nvidia model returned an empty response")
	ErrInvalidModelJSON     = errors.New("nvidia model returned invalid json")
	ErrInvalidNVIDIABaseURL = errors.New("nvidia base url is required")
)

type Config struct {
	BaseURL      string
	Token        string
	Model        string
	Timeout      time.Duration
	MaxTextRunes int
}

// Extractor calls NVIDIA's OpenAI-compatible chat completions API.
type Extractor struct {
	baseURL      string
	apiKey       string
	model        string
	timeout      time.Duration
	maxTextRunes int
	client       *http.Client
}

// NewExtractor creates an NVIDIA-backed extractor.
func NewExtractor(cfg Config) (*Extractor, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if err := validateBaseURL(baseURL); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, ErrAPIKeyRequired
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultModel
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 45 * time.Second
	}
	maxTextRunes := cfg.MaxTextRunes
	if maxTextRunes == 0 {
		maxTextRunes = 60_000
	}

	return &Extractor{
		baseURL:      baseURL,
		apiKey:       strings.TrimSpace(cfg.Token),
		model:        model,
		timeout:      timeout,
		maxTextRunes: maxTextRunes,
		client:       &http.Client{Timeout: timeout},
	}, nil
}

func (e *Extractor) Extract(ctx context.Context, req platformai.ExtractRequest) (platformai.ExtractResult, error) {
	text, err := textFromContent(req.ContentType, req.Content, e.maxTextRunes)
	if err != nil {
		return platformai.ExtractResult{}, err
	}

	body, err := json.Marshal(chatCompletionRequest{
		Model: e.model,
		Messages: []chatMessage{
			{Role: "system", Content: extractionSystemPrompt()},
			{Role: "user", Content: extractionUserPrompt(promptRequest{
				TenantID:    req.TenantID,
				DocumentID:  req.DocumentID,
				FileName:    req.FileName,
				ContentType: req.ContentType,
				SourceType:  req.SourceType,
				DraftType:   req.DraftType,
				Text:        text,
			})},
		},
		Temperature: 0,
		MaxTokens:   4096,
	})
	if err != nil {
		return platformai.ExtractResult{}, fmt.Errorf("encode nvidia request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return platformai.ExtractResult{}, fmt.Errorf("build nvidia request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	//nolint:gosec // baseURL is validated during construction and controlled by trusted runtime config.
	httpResp, err := e.client.Do(httpReq)
	if err != nil {
		return platformai.ExtractResult{}, fmt.Errorf("call nvidia api: %w", err)
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			return
		}
	}()

	respBytes, err := io.ReadAll(io.LimitReader(httpResp.Body, 4<<20))
	if err != nil {
		return platformai.ExtractResult{}, fmt.Errorf("read nvidia response: %w", err)
	}
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return platformai.ExtractResult{}, fmt.Errorf("nvidia api status %d: %s", httpResp.StatusCode, string(respBytes))
	}

	var resp chatCompletionResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return platformai.ExtractResult{}, fmt.Errorf("decode nvidia response: %w", err)
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return platformai.ExtractResult{}, ErrEmptyModelResponse
	}

	output := json.RawMessage(strings.TrimSpace(resp.Choices[0].Message.Content))
	if !json.Valid(output) {
		return platformai.ExtractResult{}, ErrInvalidModelJSON
	}

	return platformai.ExtractResult{
		Model:           e.model,
		PromptVersion:   PromptVersion,
		OutputJSON:      output,
		EvidenceJSON:    json.RawMessage("[]"),
		ConfidenceBasis: 7000,
	}, nil
}

func validateBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse nvidia base url: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return ErrInvalidNVIDIABaseURL
	}
	return nil
}

func textFromContent(contentType string, content []byte, maxRunes int) (string, error) {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))

	supported := strings.HasPrefix(mediaType, "text/") ||
		mediaType == "application/json" ||
		mediaType == "application/csv" ||
		mediaType == "text/csv"
	if !supported {
		return "", ErrUnsupportedContent
	}
	if !utf8.Valid(content) {
		return "", ErrUnsupportedContent
	}

	runes := []rune(string(content))
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes), nil
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
