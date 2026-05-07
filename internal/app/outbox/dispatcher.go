package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformkafka "github.com/deplagene/revenueleakageengine/internal/platform/kafka"
	"github.com/google/uuid"
)

const defaultRetryDelay = 30 * time.Second

var (
	ErrProducerRequired = errors.New("producer is required")
	ErrOutboxRequired   = errors.New("outbox store is required")
)

type Producer interface {
	Send(ctx context.Context, msg platformkafka.OutboundMessage) error
}

type DispatchStore interface {
	ListPending(ctx context.Context, cmd ListPendingCommand) ([]Record, error)
	MarkPublished(ctx context.Context, eventID uuid.UUID, publishedAt time.Time) error
	MarkFailed(ctx context.Context, eventID uuid.UUID, lastError string, retryAt time.Time) error
}

type Dispatcher struct {
	store      DispatchStore
	producer   Producer
	now        func() time.Time
	retryDelay time.Duration
}

type DispatcherOption func(*Dispatcher)

func WithDispatcherClock(now func() time.Time) DispatcherOption {
	return func(d *Dispatcher) {
		if now != nil {
			d.now = now
		}
	}
}

func WithRetryDelay(delay time.Duration) DispatcherOption {
	return func(d *Dispatcher) {
		if delay > 0 {
			d.retryDelay = delay
		}
	}
}

func NewDispatcher(store DispatchStore, producer Producer, opts ...DispatcherOption) (*Dispatcher, error) {
	if store == nil {
		return nil, ErrOutboxRequired
	}

	if producer == nil {
		return nil, ErrProducerRequired
	}

	dispatcher := &Dispatcher{
		store:      store,
		producer:   producer,
		now:        time.Now,
		retryDelay: defaultRetryDelay,
	}

	for _, opt := range opts {
		opt(dispatcher)
	}

	return dispatcher, nil
}

type DispatchCommand struct {
	Limit int
}

type DispatchResult struct {
	Published int
	Failed    int
}

func (d *Dispatcher) Dispatch(ctx context.Context, cmd DispatchCommand) (DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return DispatchResult{}, err
	}

	records, err := d.store.ListPending(ctx, ListPendingCommand{
		AvailableAt: d.now(),
		Limit:       cmd.Limit,
	})
	if err != nil {
		return DispatchResult{}, fmt.Errorf("list pending events: %w", err)
	}

	var result DispatchResult
	for _, record := range records {
		if err := d.publishRecord(ctx, record); err != nil {
			result.Failed++
			continue
		}

		result.Published++
	}

	return result, nil
}

func (d *Dispatcher) publishRecord(ctx context.Context, record Record) error {
	if err := d.producer.Send(ctx, platformkafka.OutboundMessage{
		Topic:   record.Topic,
		Key:     []byte(record.PartitionKey),
		Value:   record.PayloadJSON,
		Headers: byteHeaders(record.Headers),
	}); err != nil {
		if markErr := d.store.MarkFailed(ctx, record.ID, err.Error(), d.now().Add(d.retryDelay)); markErr != nil {
			return errors.Join(err, markErr)
		}

		return err
	}

	if err := d.store.MarkPublished(ctx, record.ID, d.now()); err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	return nil
}

func byteHeaders(headers map[string]string) map[string][]byte {
	if len(headers) == 0 {
		return nil
	}

	result := make(map[string][]byte, len(headers))
	for key, value := range headers {
		result[key] = []byte(value)
	}

	return result
}
