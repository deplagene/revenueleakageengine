package valueobject

import (
	"errors"
	"fmt"
)

const MaxConfidenceBasisPoints uint16 = 10_000

var ErrInvalidConfidenceScore = errors.New("invalid confidence score")

type ConfidenceScore struct {
	BasisPoints uint16
}

func NewConfidenceScore(basisPoints uint16) (ConfidenceScore, error) {
	if basisPoints > MaxConfidenceBasisPoints {
		return ConfidenceScore{}, ErrInvalidConfidenceScore
	}

	return ConfidenceScore{BasisPoints: basisPoints}, nil
}

func MustConfidenceScore(basisPoints uint16) ConfidenceScore {
	score, err := NewConfidenceScore(basisPoints)
	if err != nil {
		panic(err)
	}

	return score
}

func (s ConfidenceScore) String() string {
	return fmt.Sprintf("%d.%02d%%", s.BasisPoints/100, s.BasisPoints%100)
}
