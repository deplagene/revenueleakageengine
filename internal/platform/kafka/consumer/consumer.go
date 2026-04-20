package consumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/deplagene/revenueleakageengine/internal/platform/kafka"
)

type consumer struct {
	group       sarama.ConsumerGroup
	topics      []string
	middlewares []Middleware
}

func NewConsumer(group sarama.ConsumerGroup, topics []string, middlewares ...Middleware) *consumer {
	return &consumer{
		group:       group,
		topics:      topics,
		middlewares: middlewares,
	}
}

func (c *consumer) Consume(ctx context.Context, handler kafka.MessageHandler) error {
	const op = "platform.kafka.consumer.Consume"

	newGroupHandler := NewGroupHandler(handler, c.middlewares...)

	for {
		if err := c.group.Consume(ctx, c.topics, newGroupHandler); err != nil {
			if errors.Is(err, sarama.ErrClosedConsumerGroup) {
				return nil
			}

			// logger
			return fmt.Errorf("cannot consume: %s: %w", op, err)
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// logger
	}
}
