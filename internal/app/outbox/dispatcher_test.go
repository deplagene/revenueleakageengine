package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	appevent "github.com/deplagene/revenueleakageengine/internal/app/event"
	platformkafka "github.com/deplagene/revenueleakageengine/internal/platform/kafka"
	"github.com/google/uuid"
)

func TestDispatcherPublishesPendingEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 1, 12, 0, 0, 0, time.UTC)
	eventID := uuid.New()
	store := &fakeDispatchStore{
		records: []Record{
			{
				ID:           eventID,
				Topic:        appevent.TopicReconciliationRunCompletedV1,
				PartitionKey: "contract-1",
				PayloadJSON:  []byte(`{"event_id":"` + eventID.String() + `"}`),
				Headers: map[string]string{
					"event_id": eventID.String(),
				},
			},
		},
	}
	producer := &fakeProducer{}
	dispatcher, err := NewDispatcher(
		store,
		producer,
		WithDispatcherClock(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	result, err := dispatcher.Dispatch(context.Background(), DispatchCommand{Limit: 10})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if result.Published != 1 {
		t.Fatalf("published = %d, want 1", result.Published)
	}
	if len(producer.messages) != 1 {
		t.Fatalf("producer messages = %d, want 1", len(producer.messages))
	}
	if string(producer.messages[0].Key) != "contract-1" {
		t.Fatalf("key = %q, want contract-1", string(producer.messages[0].Key))
	}
	if store.publishedID != eventID {
		t.Fatalf("published id = %s, want %s", store.publishedID, eventID)
	}
}

func TestDispatcherMarksFailedEventsForRetry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 1, 12, 0, 0, 0, time.UTC)
	eventID := uuid.New()
	store := &fakeDispatchStore{
		records: []Record{{ID: eventID, Topic: appevent.TopicUsageRecordsV1}},
	}
	producer := &fakeProducer{err: errors.New("broker unavailable")}
	dispatcher, err := NewDispatcher(
		store,
		producer,
		WithDispatcherClock(func() time.Time { return now }),
		WithRetryDelay(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	result, err := dispatcher.Dispatch(context.Background(), DispatchCommand{Limit: 10})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if result.Failed != 1 {
		t.Fatalf("failed = %d, want 1", result.Failed)
	}
	if store.failedID != eventID {
		t.Fatalf("failed id = %s, want %s", store.failedID, eventID)
	}
	if !store.retryAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("retry at = %s, want %s", store.retryAt, now.Add(time.Minute))
	}
}

type fakeDispatchStore struct {
	records     []Record
	publishedID uuid.UUID
	failedID    uuid.UUID
	retryAt     time.Time
}

func (f *fakeDispatchStore) ListPending(
	_ context.Context,
	_ ListPendingCommand,
) ([]Record, error) {
	return f.records, nil
}

func (f *fakeDispatchStore) MarkPublished(
	_ context.Context,
	eventID uuid.UUID,
	_ time.Time,
) error {
	f.publishedID = eventID
	return nil
}

func (f *fakeDispatchStore) MarkFailed(
	_ context.Context,
	eventID uuid.UUID,
	_ string,
	retryAt time.Time,
) error {
	f.failedID = eventID
	f.retryAt = retryAt
	return nil
}

type fakeProducer struct {
	messages []platformkafka.OutboundMessage
	err      error
}

func (f *fakeProducer) Send(_ context.Context, msg platformkafka.OutboundMessage) error {
	if f.err != nil {
		return f.err
	}

	f.messages = append(f.messages, msg)
	return nil
}
