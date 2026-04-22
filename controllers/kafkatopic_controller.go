package controllers

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kafkav1 "github.com/KAnggara75/kafka-topic-controller/api/v1"
	"github.com/KAnggara75/kafka-topic-controller/kafka"
)

type KafkaTopicReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Kafka  kafka.KafkaClient
	Log    logr.Logger
}

func (r *KafkaTopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var topic kafkav1.KafkaTopic

	if err := r.Get(ctx, req.NamespacedName, &topic); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 10}, client.IgnoreNotFound(err)
	}

	actual, err := r.Kafka.GetTopic(topic.Name)
	if err != nil {
		if errors.Is(err, kafka.ErrNotFound) {
			r.Log.Info("topic not found, creating", "name", topic.Name)

			if err := r.Kafka.CreateTopic(topic.Name, topic.Spec); err != nil {
				r.Log.Error(err, "failed to create topic", "name", topic.Name)
				return ctrl.Result{}, err
			}

			r.Log.Info("topic created", "name", topic.Name)

			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		return ctrl.Result{}, err
	}

	// compare & update
	if r.Kafka.NeedsUpdate(actual, topic.Spec) {
		r.Log.Info("topic needs update", "name", topic.Name)

		if err := r.Kafka.UpdateTopic(topic.Name, topic.Spec); err != nil {
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
