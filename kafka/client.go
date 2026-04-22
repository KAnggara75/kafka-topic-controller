package kafka

import (
	"context"
	"errors"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	ctrl "sigs.k8s.io/controller-runtime"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
)

var (
	setupLog              = ctrl.Log.WithName("kafka-client")
	ErrNotFound           = errors.New("topic not found")
	errUnknownTopicOrPart = errors.New("Broker: Unknown topic or partition")
)

type TopicInfo struct {
	Name       string
	Partitions int
	Config     map[string]string
}

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
		setupLog.Info("get topic", "name", name, "err", ErrNotFound)
		return nil, ErrNotFound
	}

	topic := results.TopicDescriptions[0]

	// cek error dari broker
	if topic.Error.Code() != ckafka.ErrNoError {
		if topic.Error.Code() == ckafka.ErrUnknownTopicOrPart {
			setupLog.Info("get topic", "name", name, "result", "not found", "err", errUnknownTopicOrPart)
			return nil, ErrNotFound
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

func (k *KafkaAdminClient) CreateTopic(name string, spec kafkav1.KafkaTopicSpec) error {
	setupLog.Info("create topic",
		"name", name,
		"partitions", spec.Partitions,
		"replicationFactor", spec.ReplicationFactor,
	)

	if k == nil || k.k == nil {
		return errors.New("kafka client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	topicSpec := kafka.TopicSpecification{
		Topic:             name,
		NumPartitions:     int(spec.Partitions),
		ReplicationFactor: int(spec.ReplicationFactor),
		Config:            spec.Config,
	}

	results, err := k.k.CreateTopics(ctx, []kafka.TopicSpecification{topicSpec})
	if err != nil {
		setupLog.Error(err, "failed to create topic")
		return err
	}

	if len(results) == 0 {
		return errors.New("empty result from kafka create topic")
	}

	res := results[0]

	// handle result error
	if res.Error.Code() != kafka.ErrNoError {
		// idempotent: topic sudah ada
		if res.Error.Code() == kafka.ErrTopicAlreadyExists {
			setupLog.Info("topic already exists", "name", name)
			return nil
		}

		setupLog.Error(res.Error, "failed to create topic", "name", name)
		return res.Error
	}

	setupLog.Info("topic created successfully", "name", name)
	return nil
}

func (k *KafkaAdminClient) UpdateTopic(name string, spec kafkav1.KafkaTopicSpec) error {
	setupLog.Info("update topic", "name", name, "spec", spec)

	if k == nil || k.k == nil {
		return errors.New("kafka client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// =========================
	// 1. UPDATE CONFIG
	// =========================
	if len(spec.Config) > 0 {
		var configs []ckafka.ConfigEntry

		for key, val := range spec.Config {
			configs = append(configs, ckafka.ConfigEntry{
				Name:  key,
				Value: val,
			})
		}

		resource := ckafka.ConfigResource{
			Type:   ckafka.ResourceTopic,
			Name:   name,
			Config: configs,
		}

		results, err := k.k.AlterConfigs(ctx, []ckafka.ConfigResource{resource})
		if err != nil {
			setupLog.Error(err, "failed to alter config", "name", name)
			return err
		}

		if len(results) > 0 && results[0].Error.Code() != ckafka.ErrNoError {
			setupLog.Error(results[0].Error, "config update failed", "name", name)
			return results[0].Error
		}

		setupLog.Info("config updated", "name", name)
	}

	// =========================
	// 2. UPDATE PARTITIONS
	// =========================
	// Ambil metadata dulu
	metadata, err := k.k.GetMetadata(&name, false, 5000)
	if err != nil {
		return err
	}

	topicMetadata, ok := metadata.Topics[name]
	if !ok {
		return ErrNotFound
	}

	currentPartitions := len(topicMetadata.Partitions)
	desiredPartitions := int(spec.Partitions)

	if desiredPartitions > currentPartitions {
		setupLog.Info("increasing partitions",
			"name", name,
			"from", currentPartitions,
			"to", desiredPartitions,
		)

		_, err := k.k.CreatePartitions(ctx, []ckafka.PartitionsSpecification{
			{
				Topic:      name,
				IncreaseTo: desiredPartitions,
			},
		})

		if err != nil {
			setupLog.Error(err, "failed to increase partitions", "name", name)
			return err
		}
	}

	// =========================
	// 3. REPLICATION FACTOR
	// =========================
	if spec.ReplicationFactor > 0 {
		setupLog.Info("replication factor change is not supported automatically",
			"name", name,
			"desired", spec.ReplicationFactor,
		)
	}

	return nil
}

func (k *KafkaAdminClient) NeedsUpdate(actual any, desired kafkav1.KafkaTopicSpec) bool {
	topic, ok := actual.(TopicInfo)
	if !ok {
		setupLog.Info("invalid actual type, forcing update")
		return true
	}

	// 1. compare partitions (optional: hanya warn)
	if int(desired.Partitions) < topic.Partitions {
		setupLog.Info("partition decrease not supported",
			"current", topic.Partitions,
			"desired", desired.Partitions,
		)
	}

	if int(desired.Partitions) > topic.Partitions {
		setupLog.Info("partition increase detected",
			"current", topic.Partitions,
			"desired", desired.Partitions,
		)
		return true
	}

	// 2. compare config
	for k, v := range desired.Config {
		if topic.Config[k] != v {
			setupLog.Info("config mismatch",
				"key", k,
				"current", topic.Config[k],
				"desired", v,
			)
			return true
		}
	}

	// 3. cek config yang dihapus
	for k := range topic.Config {
		if _, ok := desired.Config[k]; !ok {
			setupLog.Info("config removed",
				"key", k,
			)
			return true
		}
	}

	return false
}
