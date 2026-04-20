package kafka

import (
	"errors"
	"strings"
	"time"
)

// ErrTopicRequired reports that an outbound Kafka message was built without a topic.
var ErrTopicRequired = errors.New("topic is required")

// Message wraps a Kafka record together with transport metadata needed by
// consumers and middleware.
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

// Validate checks that an outbound message contains the minimum transport
// metadata required for publishing.
func (m OutboundMessage) Validate() error {
	if strings.TrimSpace(m.Topic) == "" {
		return ErrTopicRequired
	}

	return nil
}
