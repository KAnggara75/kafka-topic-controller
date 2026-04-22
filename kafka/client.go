package kafka

import (
	"errors"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	ctrl "sigs.k8s.io/controller-runtime"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
)

var (
	setupLog              = ctrl.Log.WithName("kafka-client")
	ErrNotFound           = errors.New("topic not found")
	errUnknownTopicOrPart = errors.New("Broker: Unknown topic or partition")
)

type KafkaClient interface {
	GetTopic(name string) (any, error)
	CreateTopic(name string, spec kafkav1.KafkaTopicSpec) error
	UpdateTopic(name string, spec kafkav1.KafkaTopicSpec) error
	NeedsUpdate(actual any, desired kafkav1.KafkaTopicSpec) bool
}

type KafkaAdminClient struct {
	k *ckafka.AdminClient
}

func NewKafkaAdminClient(cfg *ckafka.ConfigMap) *KafkaAdminClient {
	setupLog.Info("creating kafka admin client...")
	k, err := ckafka.NewAdminClient(cfg)
	if err != nil {
		setupLog.Error(err, "failed to create kafka admin client")
		return nil
	}

	// Validate connection by fetching metadata
	setupLog.Info("validating kafka connection...")
	_, err = k.GetMetadata(nil, false, 5000)
	if err != nil {
		setupLog.Error(err, "failed to connect to kafka brokers")
		// We might still want to return the client depending on use case,
		// but here we fail fast.
		return nil
	}

	setupLog.Info("successfully connected to kafka")
	return &KafkaAdminClient{
		k: k,
	}
}
