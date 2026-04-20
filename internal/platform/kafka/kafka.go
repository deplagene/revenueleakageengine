package kafka

import "context"

type MessageHandler func(ctx context.Context, msg Message) error

type Consumer interface {
	Consume(ctx context.Context, handler MessageHandler) error
}

type Producer interface {
	Send(ctx context.Context, msg OutboundMessage) error
}
