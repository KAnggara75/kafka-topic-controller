package kafka

import (
	"errors"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	ctrl "sigs.k8s.io/controller-runtime"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
)

var (
	setupLog    = ctrl.Log.WithName("kafka-client")
	errNotFound = errors.New("topic not found")
)

type KafkaClient interface {
	GetTopic(name string) (any, error)
	CreateTopic(spec kafkav1.KafkaTopicSpec) error
	UpdateTopic(spec kafkav1.KafkaTopicSpec) error
	NeedsUpdate(actual any, desired kafkav1.KafkaTopicSpec) bool
}

type KafkaAdminClient struct {
	k *kafka.AdminClient
}

func NewKafkaAdminClient(cfg *kafka.ConfigMap) *KafkaAdminClient {
	k, err := kafka.NewAdminClient(cfg)
	if err != nil {
		setupLog.Error(err, "failed to create kafka admin client")
		return nil
	}

	return &KafkaAdminClient{
		k: k,
	}
}

func (k *KafkaAdminClient) GetTopic(name string) (any, error) {
	setupLog.Info("get topic", "name", name)
	// TODO: implement kafka admin client
	return nil, errNotFound
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
