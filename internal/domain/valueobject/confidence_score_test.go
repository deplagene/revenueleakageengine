package valueobject

import "testing"

func TestConfidenceScoreString(t *testing.T) {
	score := MustConfidenceScore(8_750)

	if got := score.String(); got != "87.50%" {
		t.Fatalf("unexpected score string: %s", got)
	}
}

func TestNewConfidenceScoreRejectsOutOfRange(t *testing.T) {
	if _, err := NewConfidenceScore(MaxConfidenceBasisPoints + 1); err == nil {
		t.Fatal("expected invalid confidence score error")
	}
}
