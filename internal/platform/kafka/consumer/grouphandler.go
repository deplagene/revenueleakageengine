package consumer

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/deplagene/revenueleakageengine/internal/platform/kafka"
)

type Middleware func(next kafka.MessageHandler) kafka.MessageHandler

type groupHandler struct {
	handler kafka.MessageHandler
}

func NewGroupHandler(handler kafka.MessageHandler, middlewares ...Middleware) *groupHandler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return &groupHandler{
		handler: handler,
	}
}

func (g *groupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (g *groupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// TODO: add logging
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

func extractHeaders(headers []*sarama.RecordHeader) map[string][]byte {
	result := make(map[string][]byte)
	for _, h := range headers {
		if h != nil && h.Key != nil {
			result[string(h.Key)] = h.Value
		}
	}

	return result
}
