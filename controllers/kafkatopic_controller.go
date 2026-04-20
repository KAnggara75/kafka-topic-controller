package controllers

import (
	"context"
	"time"

	"github.com/segmentio/topicctl/pkg/admin"
	"github.com/segmentio/topicctl/pkg/apply"
	"github.com/segmentio/topicctl/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kafkav1alpha1 "github.com/KAnggara75/kafka-topic-controller/api/v1alpha1"
)

const kafkaTopicFinalizer = "kafkatopic.topicctl.kafka.kanggara.my.id/finalizer"

// KafkaTopicReconciler reconciles a KafkaTopic object
type KafkaTopicReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	BootstrapServers string
}

//+kubebuilder:rbac:groups=kafka.kanggara.my.id,resources=kafkatopics,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kafka.kanggara.my.id,resources=kafkatopics/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kafka.kanggara.my.id,resources=kafkatopics/finalizers,verbs=update

func (r *KafkaTopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var kafkaTopic kafkav1alpha1.KafkaTopic
	if err := r.Get(ctx, req.NamespacedName, &kafkaTopic); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !kafkaTopic.DeletionTimestamp.IsZero() {
		l.Info("Deleting Kafka topic", "name", kafkaTopic.Name)
		if controllerutil.ContainsFinalizer(&kafkaTopic, kafkaTopicFinalizer) {
			if err := r.deleteKafkaTopic(ctx, &kafkaTopic); err != nil {
				l.Error(err, "Failed to delete kafka topic")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&kafkaTopic, kafkaTopicFinalizer)
			if err := r.Update(ctx, &kafkaTopic); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(&kafkaTopic, kafkaTopicFinalizer) {
		controllerutil.AddFinalizer(&kafkaTopic, kafkaTopicFinalizer)
		if err := r.Update(ctx, &kafkaTopic); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Sync with Kafka
	l.Info("Syncing Kafka topic", "name", kafkaTopic.Name)
	if err := r.syncKafkaTopic(ctx, &kafkaTopic); err != nil {
		l.Error(err, "Failed to sync kafka topic")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

func (r *KafkaTopicReconciler) deleteKafkaTopic(ctx context.Context, kt *kafkav1alpha1.KafkaTopic) error {
	adminClient, err := admin.NewBrokerAdminClient(ctx, admin.BrokerAdminClientConfig{
		ConnectorConfig: admin.ConnectorConfig{
			BrokerAddr: r.BootstrapServers,
		},
	})
	if err != nil {
		return err
	}
	defer adminClient.Close()

	return adminClient.DeleteTopic(ctx, kt.Name)
}

func (r *KafkaTopicReconciler) syncKafkaTopic(ctx context.Context, kt *kafkav1alpha1.KafkaTopic) error {
	adminClient, err := admin.NewBrokerAdminClient(ctx, admin.BrokerAdminClientConfig{
		ConnectorConfig: admin.ConnectorConfig{
			BrokerAddr: r.BootstrapServers,
		},
	})
	if err != nil {
		return err
	}
	defer adminClient.Close()

	// Map K8s spec to topicctl config
	topicConfig := config.TopicConfig{
		Meta: config.ResourceMeta{
			Name: kt.Name,
		},
		Spec: config.TopicSpec{
			Partitions:        kt.Spec.Partitions,
			ReplicationFactor: kt.Spec.ReplicationFactor,
			Settings:          config.FromConfigMap(kt.Spec.Settings),
			RetentionMinutes:  kt.Spec.RetentionMinutes,
		},
	}

	// Use apply.NewTopicApplier to perform idempotent updates
	applier, err := apply.NewTopicApplier(
		ctx,
		adminClient,
		apply.TopicApplierConfig{
			ClusterConfig: config.ClusterConfig{},
			TopicConfig:   topicConfig,
			DryRun:        false,
		},
	)
	if err != nil {
		return err
	}

	return applier.Apply(ctx)
}

func (r *KafkaTopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1alpha1.KafkaTopic{}).
		Complete(r)
}
