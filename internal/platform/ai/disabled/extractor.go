package disabled

import (
	"context"
	"errors"

	platformai "github.com/deplagene/revenueleakageengine/internal/platform/ai"
)

var ErrExtractorDisabled = errors.New("ai extractor is disabled: configure RLE_AI_PROVIDER=nvidia and RLE_NVIDIA_API_KEY")

// Extractor rejects extraction while preserving application startup without AI
// credentials.
type Extractor struct{}

func (Extractor) Extract(context.Context, platformai.ExtractRequest) (platformai.ExtractResult, error) {
	return platformai.ExtractResult{}, ErrExtractorDisabled
}
