package kafka

import (
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
)

func TestKafkaAdminClient_New(t *testing.T) {
	// This might fail if librdkafka is not installed in the environment,
	// but we can try to test the constructor logic.
	cfg := &kafka.ConfigMap{
		"bootstrap.servers": "localhost:9092",
	}
	client := NewKafkaAdminClient(cfg)
	if client == nil {
		t.Skip("Skipping test: Kafka admin client could not be created (likely missing librdkafka)")
	}
}

func TestKafkaAdminClient_Methods(t *testing.T) {
	// Since the methods are placeholders, we just verify they don't panic
	client := &KafkaAdminClient{k: nil}

	_, err := client.GetTopic("test")
	if err != ErrNotFound {
		t.Errorf("expected errNotFound, got %v", err)
	}

	err = client.CreateTopic("test", kafkav1.KafkaTopicSpec{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	err = client.UpdateTopic("test", kafkav1.KafkaTopicSpec{})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	needsUpdate := client.NeedsUpdate(nil, kafkav1.KafkaTopicSpec{})
	if !needsUpdate {
		t.Error("expected true, got false")
	}
}
