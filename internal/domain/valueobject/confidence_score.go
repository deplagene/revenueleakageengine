package valueobject

import (
	"errors"
	"fmt"
)

// MaxConfidenceBasisPoints is the maximum supported confidence score value,
// representing exactly 100.00%.
const MaxConfidenceBasisPoints uint16 = 10_000

// ErrInvalidConfidenceScore reports that a confidence score falls outside the
// supported 0 to 100.00% range.
var ErrInvalidConfidenceScore = errors.New("invalid confidence score")

// ConfidenceScore represents a confidence percentage in basis points to avoid
// floating-point ambiguity.
type ConfidenceScore struct {
	BasisPoints uint16
}

// NewConfidenceScore constructs a confidence score and rejects values above
// 100.00%.
func NewConfidenceScore(basisPoints uint16) (ConfidenceScore, error) {
	if basisPoints > MaxConfidenceBasisPoints {
		return ConfidenceScore{}, ErrInvalidConfidenceScore
	}

	return ConfidenceScore{BasisPoints: basisPoints}, nil
}

// MustConfidenceScore constructs a confidence score and panics when the value
// is invalid. Use it only for trusted literals and test fixtures.
func MustConfidenceScore(basisPoints uint16) ConfidenceScore {
	score, err := NewConfidenceScore(basisPoints)
	if err != nil {
		panic(err)
	}

	return score
}

// String formats the confidence score as a percentage with two decimal places.
func (s ConfidenceScore) String() string {
	return fmt.Sprintf("%d.%02d%%", s.BasisPoints/100, s.BasisPoints%100)
}
