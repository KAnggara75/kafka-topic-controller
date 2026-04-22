package kafka

import (
	"context"
	"errors"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
)

type TopicInfo struct {
	Name       string
	Partitions int
	Config     map[string]string
}

func (k *KafkaAdminClient) GetTopic(name string) (any, error) {
	if k == nil || k.k == nil {
		return nil, errors.New("kafka client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Describe topic (partitions)
	results, err := k.k.DescribeTopics(
		ctx,
		ckafka.NewTopicCollectionOfTopicNames([]string{name}),
	)
	if err != nil {
		setupLog.Error(err, "failed to describe topic")
		return nil, err
	}

	if len(results.TopicDescriptions) == 0 {
		return nil, ErrNotFound
	}

	topicDesc := results.TopicDescriptions[0]

	// cek error dari broker
	if topicDesc.Error.Code() != ckafka.ErrNoError {
		if topicDesc.Error.Code() == ckafka.ErrUnknownTopicOrPart {
			return nil, ErrNotFound
		}
		return nil, topicDesc.Error
	}

	// 2. Describe Configs
	configRes, err := k.k.DescribeConfigs(ctx, []ckafka.ConfigResource{
		{
			Type: ckafka.ResourceTopic,
			Name: name,
		},
	})
	if err != nil {
		setupLog.Error(err, "failed to describe configs", "name", name)
		return nil, err
	}

	configMap := make(map[string]string)
	if len(configRes) > 0 {
		for _, entry := range configRes[0].Config {
			// Kita hanya ambil yang bukan default atau yang relevan?
			// Biasanya kita bandingkan semua yang di-set.
			// Tapi Kafka punya banyak default.
			// Strategi: ambil semua, tapi nanti NeedsUpdate hanya cek yang ada di 'desired'.
			configMap[entry.Name] = entry.Value
		}
	}

	info := TopicInfo{
		Name:       topicDesc.Name,
		Partitions: len(topicDesc.Partitions),
		Config:     normalizeConfig(configMap),
	}

	return info, nil
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

	topicSpec := ckafka.TopicSpecification{
		Topic:             name,
		NumPartitions:     int(spec.Partitions),
		ReplicationFactor: int(spec.ReplicationFactor),
		Config:            spec.Config,
	}

	results, err := k.k.CreateTopics(ctx, []ckafka.TopicSpecification{topicSpec})
	if err != nil {
		setupLog.Error(err, "failed to create topic")
		return err
	}

	if len(results) == 0 {
		return errors.New("empty result from kafka create topic")
	}

	res := results[0]

	// handle result error
	if res.Error.Code() != ckafka.ErrNoError {
		// idempotent: topic sudah ada
		if res.Error.Code() == ckafka.ErrTopicAlreadyExists {
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
			"name", topic.Name,
			"current", topic.Partitions,
			"desired", desired.Partitions,
		)
	}

	if int(desired.Partitions) > topic.Partitions {
		setupLog.Info("partition increase detected",
			"name", topic.Name,
			"current", topic.Partitions,
			"desired", desired.Partitions,
		)
		return true
	}

	// 2. compare config
	desiredConfig := normalizeConfig(desired.Config)
	for key, desiredVal := range desiredConfig {
		actualVal, exists := topic.Config[key]
		if !exists || actualVal != desiredVal {
			setupLog.Info("config mismatch",
				"name", topic.Name,
				"key", key,
				"current", actualVal,
				"desired", desiredVal,
			)
			return true
		}
	}

	// 3. (Optional) cek config yang dihapus?
	// Tergantung kebijakan: apakah kita mau hapus config yang tidak ada di desired?
	// Untuk sekarang, kita hanya pastikan yang di desired terpenuhi.

	return false
}
