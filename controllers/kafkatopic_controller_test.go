package controllers

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
)

type MockKafkaClient struct {
	GetTopicFn    func(name string) (any, error)
	CreateTopicFn func(name string, spec kafkav1.KafkaTopicSpec) error
	UpdateTopicFn func(name string, spec kafkav1.KafkaTopicSpec) error
	NeedsUpdateFn func(actual any, desired kafkav1.KafkaTopicSpec) bool
}

func (m *MockKafkaClient) GetTopic(name string) (any, error) { return m.GetTopicFn(name) }
func (m *MockKafkaClient) CreateTopic(name string, spec kafkav1.KafkaTopicSpec) error {
	return m.CreateTopicFn(name, spec)
}
func (m *MockKafkaClient) UpdateTopic(name string, spec kafkav1.KafkaTopicSpec) error {
	return m.UpdateTopicFn(name, spec)
}
func (m *MockKafkaClient) NeedsUpdate(actual any, desired kafkav1.KafkaTopicSpec) bool {
	return m.NeedsUpdateFn(actual, desired)
}

func TestKafkaTopicReconciler_Reconcile_Create(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = kafkav1.AddToScheme(scheme)

	topic := &kafkav1.KafkaTopic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-topic",
			Namespace: "default",
		},
		Spec: kafkav1.KafkaTopicSpec{
			Partitions:        3,
			ReplicationFactor: 1,
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(topic).Build()

	created := false
	mockKafka := &MockKafkaClient{
		GetTopicFn: func(name string) (any, error) {
			return nil, errors.New("not found")
		},
		CreateTopicFn: func(name string, spec kafkav1.KafkaTopicSpec) error {
			created = true
			return nil
		},
	}

	r := &KafkaTopicReconciler{
		Client: cl,
		Scheme: scheme,
		Kafka:  mockKafka,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-topic",
			Namespace: "default",
		},
	}

	_, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if !created {
		t.Error("expected topic to be created, but it wasn't")
	}
}

func TestKafkaTopicReconciler_Reconcile_Update(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = kafkav1.AddToScheme(scheme)

	topic := &kafkav1.KafkaTopic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-topic",
			Namespace: "default",
		},
		Spec: kafkav1.KafkaTopicSpec{
			Partitions:        3,
			ReplicationFactor: 1,
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(topic).Build()

	updated := false
	mockKafka := &MockKafkaClient{
		GetTopicFn: func(name string) (any, error) {
			return "existing-topic", nil
		},
		NeedsUpdateFn: func(actual any, desired kafkav1.KafkaTopicSpec) bool {
			return true
		},
		UpdateTopicFn: func(name string, spec kafkav1.KafkaTopicSpec) error {
			updated = true
			return nil
		},
	}

	r := &KafkaTopicReconciler{
		Client: cl,
		Scheme: scheme,
		Kafka:  mockKafka,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-topic",
			Namespace: "default",
		},
	}

	_, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if !updated {
		t.Error("expected topic to be updated, but it wasn't")
	}
}
