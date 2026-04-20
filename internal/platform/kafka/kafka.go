package kafka

import "context"

// MessageHandler processes a consumed Kafka message inside the current consumer
// session context.
type MessageHandler func(ctx context.Context, msg Message) error

// Consumer describes a Kafka consumer group adapter that pulls messages and
// forwards them to a handler.
type Consumer interface {
	Consume(ctx context.Context, handler MessageHandler) error
}

// Producer describes a Kafka publisher that sends outbound messages.
type Producer interface {
	Send(ctx context.Context, msg OutboundMessage) error
}
