// Package producer provides a Sarama-backed Kafka publisher implementation.
package producer

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/deplagene/revenueleakageengine/internal/platform/kafka"
)

type producer struct {
	syncProducer sarama.SyncProducer
}

// NewProducer builds a synchronous Kafka producer backed by Sarama.
func NewProducer(syncProducer sarama.SyncProducer) *producer {
	return &producer{
		syncProducer: syncProducer,
	}
}

// Send validates an outbound message, checks the context before publish, and
// forwards the record to Kafka with headers preserved.
func (p *producer) Send(ctx context.Context, msg kafka.OutboundMessage) error {
	const op = "platform.kafka.producer.Send"

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context error before send: %s: %w", op, err)
	}

	if err := msg.Validate(); err != nil {
		return fmt.Errorf("could not validate outbound message: %s: %w", op, err)
	}

	// TODO: use partition/offset and logging when publishing metrics/traces.
	_, _, err := p.syncProducer.SendMessage(&sarama.ProducerMessage{
		Topic:   msg.Topic,
		Key:     sarama.ByteEncoder(msg.Key),
		Value:   sarama.ByteEncoder(msg.Value),
		Headers: toRecordHeaders(msg.Headers),
	})
	if err != nil {
		return fmt.Errorf("could not send message: %s: %w", op, err)
	}

	// TODO: logger

	return nil
}

// toRecordHeaders converts header maps into the Sarama representation used for
// outbound Kafka messages.
func toRecordHeaders(headers map[string][]byte) []sarama.RecordHeader {
	if len(headers) == 0 {
		return nil
	}

	result := make([]sarama.RecordHeader, 0, len(headers))
	for key, value := range headers {
		result = append(result, sarama.RecordHeader{
			Key:   []byte(key),
			Value: value,
		})
	}

	return result
}
