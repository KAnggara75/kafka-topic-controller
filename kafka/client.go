package kafka

import (
	"errors"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	setupLog    = ctrl.Log.WithName("kafka-client")
	errNotFound = errors.New("topic not found")
)

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
	// TODO: implement kafka admin client
	return nil, errNotFound
}

func (k *KafkaAdminClient) CreateTopic(spec any) error {
	// TODO: call kafka create topic
	return nil
}

func (k *KafkaAdminClient) UpdateTopic(spec any) error {
	// TODO: update config
	return nil
}

func (k *KafkaAdminClient) NeedsUpdate(actual any, desired any) bool {
	return true
}
