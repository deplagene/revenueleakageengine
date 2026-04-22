package producer

import (
	"context"
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/deplagene/revenueleakageengine/internal/platform/kafka"
)

func TestProducerSendPublishesTopicKeyValueAndHeaders(t *testing.T) {
	fake := &fakeSyncProducer{}
	producer := NewProducer(fake)

	err := producer.Send(context.Background(), kafka.OutboundMessage{
		Topic: "normalized.invoices",
		Key:   []byte("tenant-1:contract-42"),
		Value: []byte(`{"id":"inv-1"}`),
		Headers: map[string][]byte{
			"tenant_id":         []byte("tenant-1"),
			"business_trace_id": []byte("trace-99"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.lastMessage == nil {
		t.Fatal("expected producer message to be sent")
	}

	if fake.lastMessage.Topic != "normalized.invoices" {
		t.Fatalf("unexpected topic: %s", fake.lastMessage.Topic)
	}

	if got, _ := fake.lastMessage.Key.Encode(); string(got) != "tenant-1:contract-42" {
		t.Fatalf("unexpected key: %s", string(got))
	}

	if got, _ := fake.lastMessage.Value.Encode(); string(got) != `{"id":"inv-1"}` {
		t.Fatalf("unexpected value: %s", string(got))
	}

	if len(fake.lastMessage.Headers) != 2 {
		t.Fatalf("unexpected header count: %d", len(fake.lastMessage.Headers))
	}
}

func TestProducerSendRejectsEmptyTopic(t *testing.T) {
	producer := NewProducer(&fakeSyncProducer{})

	if err := producer.Send(context.Background(), kafka.OutboundMessage{}); err == nil {
		t.Fatal("expected validation error")
	}
}

type fakeSyncProducer struct {
	lastMessage *sarama.ProducerMessage
	sendErr     error
}

func (f *fakeSyncProducer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	f.lastMessage = msg
	return 0, 0, f.sendErr
}

func (f *fakeSyncProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	return nil
}

func (f *fakeSyncProducer) Close() error {
	return nil
}

func (f *fakeSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag {
	return sarama.ProducerTxnFlagReady
}

func (f *fakeSyncProducer) IsTransactional() bool {
	return false
}

func (f *fakeSyncProducer) BeginTxn() error {
	return nil
}

func (f *fakeSyncProducer) CommitTxn() error {
	return nil
}

func (f *fakeSyncProducer) AbortTxn() error {
	return nil
}

func (f *fakeSyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}

func (f *fakeSyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}

var _ sarama.SyncProducer = (*fakeSyncProducer)(nil)

func TestProducerSendReturnsBrokerError(t *testing.T) {
	producer := NewProducer(&fakeSyncProducer{sendErr: errors.New("broker unavailable")})

	if err := producer.Send(context.Background(), kafka.OutboundMessage{Topic: "revenue.expected"}); err == nil {
		t.Fatal("expected broker error")
	}
}

func TestProducerSendRejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	producer := NewProducer(&fakeSyncProducer{})

	if err := producer.Send(ctx, kafka.OutboundMessage{Topic: "revenue.expected"}); err == nil {
		t.Fatal("expected context cancellation error")
	}
}
