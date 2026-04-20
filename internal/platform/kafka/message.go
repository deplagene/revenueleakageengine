package kafka

import (
	"errors"
	"strings"
	"time"
)

var ErrTopicRequired = errors.New("topic is required")

// Message — универсальная обёртка над сообщением Kafka.
type Message struct {
	Headers        map[string][]byte
	Timestamp      time.Time
	BlockTimestamp time.Time

	Key       []byte
	Value     []byte
	Topic     string
	Partition int32
	Offset    int64
}

// OutboundMessage describes a message that should be published to Kafka.
type OutboundMessage struct {
	Headers map[string][]byte
	Key     []byte
	Value   []byte
	Topic   string
}

func (m OutboundMessage) Validate() error {
	if strings.TrimSpace(m.Topic) == "" {
		return ErrTopicRequired
	}

	return nil
}
