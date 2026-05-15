package controllers

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
	"github.com/KAnggara75/kafka-topic-controller/config"
	"github.com/KAnggara75/kafka-topic-controller/kafka"
)

type KafkaTopicReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger

	mu      sync.RWMutex
	clients map[string]kafka.KafkaClient
}

func (r *KafkaTopicReconciler) getKafkaClient(clusterUrl string) kafka.KafkaClient {
	r.mu.RLock()
	if client, ok := r.clients[clusterUrl]; ok {
		r.mu.RUnlock()
		return client
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check
	if client, ok := r.clients[clusterUrl]; ok {
		return client
	}

	if r.clients == nil {
		r.clients = make(map[string]kafka.KafkaClient)
	}

	cfg := config.GetBaseKafkaConfig(clusterUrl)
	newClient := kafka.NewKafkaAdminClient(cfg)
	if newClient != nil {
		r.clients[clusterUrl] = newClient
	}
	return newClient
}

func (r *KafkaTopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var topic kafkav1.KafkaTopic

	if err := r.Get(ctx, req.NamespacedName, &topic); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 10}, client.IgnoreNotFound(err)
	}

	kafkaClient := r.getKafkaClient(topic.Spec.ClusterUrl)
	if kafkaClient == nil {
		r.Log.Error(nil, "failed to get kafka client for cluster", "clusterUrl", topic.Spec.ClusterUrl)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	actual, err := kafkaClient.GetTopic(topic.Name)
	if err != nil {
		if errors.Is(err, kafka.ErrNotFound) {
			r.Log.Info("topic not found, creating", "name", topic.Name)

			if err := kafkaClient.CreateTopic(topic.Name, topic.Spec); err != nil {
				r.Log.Error(err, "failed to create topic", "name", topic.Name)
				return ctrl.Result{}, err
			}

			r.Log.Info("topic created", "name", topic.Name)

			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		return ctrl.Result{}, err
	}

	// compare & update
	if kafkaClient.NeedsUpdate(actual, topic.Spec) {
		r.Log.Info("topic needs update", "name", topic.Name)

		if err := kafkaClient.UpdateTopic(topic.Name, topic.Spec); err != nil {
			r.Log.Error(err, "failed to update topic", "name", topic.Name)
			return ctrl.Result{}, err
		}

		r.Log.Info("topic updated", "name", topic.Name)

		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

func (r *KafkaTopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1.KafkaTopic{}).
		Complete(r)
}
