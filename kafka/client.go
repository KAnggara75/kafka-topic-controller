package kafka

import (
	"context"
	"errors"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	ctrl "sigs.k8s.io/controller-runtime"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
)

var (
	setupLog              = ctrl.Log.WithName("kafka-client")
	errNotFound           = errors.New("topic not found")
	errUnknownTopicOrPart = errors.New("Broker: Unknown topic or partition")
)

type KafkaClient interface {
	GetTopic(name string) (any, error)
	CreateTopic(spec kafkav1.KafkaTopicSpec) error
	UpdateTopic(spec kafkav1.KafkaTopicSpec) error
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

func (k *KafkaAdminClient) GetTopic(name string) (any, error) {

	if k == nil || k.k == nil {
		return nil, errors.New("kafka client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Describe topic
	results, err := k.k.DescribeTopics(
		ctx,
		ckafka.NewTopicCollectionOfTopicNames([]string{name}),
	)
	if err != nil {
		setupLog.Error(err, "failed to describe topic")
		return nil, err
	}

	if len(results.TopicDescriptions) == 0 {
		setupLog.Info("get topic", "name", name, "err", errNotFound)
		return nil, errNotFound
	}

	topic := results.TopicDescriptions[0]

	// cek error dari broker
	if topic.Error.Code() != ckafka.ErrNoError {
		if topic.Error.Code() == ckafka.ErrUnknownTopicOrPart {
			setupLog.Info("get topic", "name", name, "result", "not found", "err", errUnknownTopicOrPart)
			return nil, errNotFound
		}
		setupLog.Info("get topic", "name", name, "err", topic.Error)
		return nil, topic.Error
	}

	setupLog.Info("topic found",
		"name", topic.Name,
		"partitions", len(topic.Partitions),
	)

	return topic, nil
}

func (k *KafkaAdminClient) CreateTopic(spec kafkav1.KafkaTopicSpec) error {
	setupLog.Info("create topic", "spec", spec)
	// TODO: call kafka create topic
	return nil
}

func (k *KafkaAdminClient) UpdateTopic(spec kafkav1.KafkaTopicSpec) error {
	setupLog.Info("update topic", "spec", spec)
	// TODO: update config
	return nil
}

func (k *KafkaAdminClient) NeedsUpdate(actual any, desired kafkav1.KafkaTopicSpec) bool {
	setupLog.Info("needs update", "actual", actual, "desired", desired)
	return true
}
