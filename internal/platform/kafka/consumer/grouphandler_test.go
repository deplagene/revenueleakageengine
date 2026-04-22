package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/deplagene/revenueleakageengine/internal/platform/kafka"
)

func TestGroupHandlerMarksMessageOnSuccess(t *testing.T) {
	session := newFakeConsumerGroupSession()
	claim := newFakeClaim(&sarama.ConsumerMessage{
		Topic:     "normalized.contracts",
		Partition: 3,
		Offset:    11,
		Key:       []byte("contract-1"),
		Value:     []byte(`{"id":"contract-1"}`),
	})

	handler := NewGroupHandler(func(ctx context.Context, msg kafka.Message) error {
		if msg.Topic != "normalized.contracts" {
			t.Fatalf("unexpected topic: %s", msg.Topic)
		}

		return nil
	})

	if err := handler.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(session.marked) != 1 {
		t.Fatalf("expected 1 marked message, got %d", len(session.marked))
	}
}

func TestGroupHandlerDoesNotMarkMessageOnFailure(t *testing.T) {
	session := newFakeConsumerGroupSession()
	claim := newFakeClaim(&sarama.ConsumerMessage{
		Topic:     "normalized.usage-records",
		Partition: 5,
		Offset:    27,
		Key:       []byte("usage-1"),
		Value:     []byte(`{"id":"usage-1"}`),
	})

	handler := NewGroupHandler(func(ctx context.Context, msg kafka.Message) error {
		return errors.New("processing failed")
	})

	if err := handler.ConsumeClaim(session, claim); err == nil {
		t.Fatal("expected handler error")
	}

	if len(session.marked) != 0 {
		t.Fatalf("expected no marked messages, got %d", len(session.marked))
	}
}

type fakeConsumerGroupSession struct {
	ctx    context.Context
	marked []*sarama.ConsumerMessage
}

func newFakeConsumerGroupSession() *fakeConsumerGroupSession {
	return &fakeConsumerGroupSession{ctx: context.Background()}
}

func (f *fakeConsumerGroupSession) Claims() map[string][]int32 {
	return nil
}

func (f *fakeConsumerGroupSession) MemberID() string {
	return "member-1"
}

func (f *fakeConsumerGroupSession) GenerationID() int32 {
	return 1
}

func (f *fakeConsumerGroupSession) MarkOffset(string, int32, int64, string) {}

func (f *fakeConsumerGroupSession) Commit() {}

func (f *fakeConsumerGroupSession) ResetOffset(string, int32, int64, string) {}

func (f *fakeConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	f.marked = append(f.marked, msg)
}

func (f *fakeConsumerGroupSession) Context() context.Context {
	return f.ctx
}

type fakeClaim struct {
	messages chan *sarama.ConsumerMessage
}

func newFakeClaim(messages ...*sarama.ConsumerMessage) *fakeClaim {
	ch := make(chan *sarama.ConsumerMessage, len(messages))
	for _, message := range messages {
		ch <- message
	}
	close(ch)

	return &fakeClaim{messages: ch}
}

func (f *fakeClaim) Topic() string {
	return "test-topic"
}

func (f *fakeClaim) Partition() int32 {
	return 0
}

func (f *fakeClaim) InitialOffset() int64 {
	return 0
}

func (f *fakeClaim) HighWaterMarkOffset() int64 {
	return 0
}

func (f *fakeClaim) Messages() <-chan *sarama.ConsumerMessage {
	return f.messages
}

var (
	_ sarama.ConsumerGroupSession = (*fakeConsumerGroupSession)(nil)
	_ sarama.ConsumerGroupClaim   = (*fakeClaim)(nil)
)

func TestExtractHeaders(t *testing.T) {
	headers := extractHeaders([]*sarama.RecordHeader{
		{Key: []byte("tenant_id"), Value: []byte("tenant-1")},
		{Key: []byte("business_trace_id"), Value: []byte("trace-77")},
	})

	if got := string(headers["tenant_id"]); got != "tenant-1" {
		t.Fatalf("unexpected tenant_id header: %s", got)
	}

	if got := string(headers["business_trace_id"]); got != "trace-77" {
		t.Fatalf("unexpected business_trace_id header: %s", got)
	}
}

func TestGroupHandlerPassesHeadersToHandler(t *testing.T) {
	session := newFakeConsumerGroupSession()
	claim := newFakeClaim(&sarama.ConsumerMessage{
		Topic:     "leakage.detected",
		Partition: 1,
		Offset:    99,
		Timestamp: time.Now(),
		Headers: []*sarama.RecordHeader{
			{Key: []byte("tenant_id"), Value: []byte("tenant-1")},
		},
	})

	handler := NewGroupHandler(func(ctx context.Context, msg kafka.Message) error {
		if got := string(msg.Headers["tenant_id"]); got != "tenant-1" {
			t.Fatalf("unexpected tenant_id header: %s", got)
		}

		return nil
	})

	if err := handler.ConsumeClaim(session, claim); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
