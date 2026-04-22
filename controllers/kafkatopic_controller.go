package controllers

import (
	"context"
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
		// topic belum ada → create
		err = r.Kafka.CreateTopic(topic.Name, topic.Spec)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// compare & update
	if r.Kafka.NeedsUpdate(actual, topic.Spec) {
		err = r.Kafka.UpdateTopic(topic.Spec)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
	}

	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

func (r *KafkaTopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1.KafkaTopic{}).
		Complete(r)
}
