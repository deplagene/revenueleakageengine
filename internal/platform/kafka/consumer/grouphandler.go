package consumer

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/deplagene/revenueleakageengine/internal/platform/kafka"
)

// Middleware wraps a message handler with transport-level behavior such as
// tracing, metrics, or panic recovery.
type Middleware func(next kafka.MessageHandler) kafka.MessageHandler

type groupHandler struct {
	handler kafka.MessageHandler
}

// NewGroupHandler constructs a Sarama consumer-group handler and applies
// middleware in reverse registration order.
func NewGroupHandler(handler kafka.MessageHandler, middlewares ...Middleware) *groupHandler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return &groupHandler{
		handler: handler,
	}
}

// Setup is called by Sarama when a new consumer group session starts.
func (g *groupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is called by Sarama before the current consumer group session ends.
func (g *groupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim reads messages from a claimed partition, forwards them to the
// handler, and marks only successfully processed messages.
func (g *groupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	const op = "platform.kafka.consumer.grouphandler"

	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				// logger
				return nil
			}

			msg := kafka.Message{
				Key:            message.Key,
				Value:          message.Value,
				Topic:          message.Topic,
				Partition:      message.Partition,
				Offset:         message.Offset,
				Timestamp:      message.Timestamp,
				BlockTimestamp: message.BlockTimestamp,
				Headers:        extractHeaders(message.Headers),
			}

			if err := g.handler(session.Context(), msg); err != nil {
				return fmt.Errorf(
					"handler failed: %s: topic=%s partition=%d offset=%d: %w",
					op,
					message.Topic,
					message.Partition,
					message.Offset,
					err,
				)
			}

			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// extractHeaders converts Kafka record headers into a map for easier handler
// consumption.
func extractHeaders(headers []*sarama.RecordHeader) map[string][]byte {
	result := make(map[string][]byte)
	for _, h := range headers {
		if h != nil && h.Key != nil {
			result[string(h.Key)] = h.Value
		}
	}

	return result
}
