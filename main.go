package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kafkav1alpha1 "github.com/KAnggara75/kafka-topic-controller/api/v1alpha1"
	"github.com/KAnggara75/kafka-topic-controller/controllers"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kafkav1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var kafkaBootstrapServers string

	var kafkaSASLMechanism string
	var kafkaSASLUser string
	var kafkaSASLPassword string
	var kafkaTLSEnabled bool
	var kafkaTLSSkipVerify bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&kafkaBootstrapServers, "kafka-bootstrap-servers", os.Getenv("KAFKA_BOOTSTRAP_SERVERS"), "Kafka bootstrap servers")
	flag.StringVar(&kafkaSASLMechanism, "kafka-sasl-mechanism", os.Getenv("KAFKA_SASL_MECHANISM"), "Kafka SASL mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)")
	flag.StringVar(&kafkaSASLUser, "kafka-sasl-user", os.Getenv("KAFKA_SASL_USER"), "Kafka SASL user")
	flag.StringVar(&kafkaSASLPassword, "kafka-sasl-password", os.Getenv("KAFKA_SASL_PASSWORD"), "Kafka SASL password")
	flag.BoolVar(&kafkaTLSEnabled, "kafka-tls-enabled", os.Getenv("KAFKA_TLS_ENABLED") == "true", "Enable Kafka TLS")
	flag.BoolVar(&kafkaTLSSkipVerify, "kafka-tls-skip-verify", os.Getenv("KAFKA_TLS_SKIP_VERIFY") == "true", "Skip Kafka TLS verification")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if kafkaBootstrapServers == "" {
		setupLog.Error(nil, "kafka-bootstrap-servers must be set via flag or KAFKA_BOOTSTRAP_SERVERS env var")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "kafka-topic-controller-leader-election",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.KafkaTopicReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		BootstrapServers: kafkaBootstrapServers,
		SASLMechanism:    kafkaSASLMechanism,
		SASLUser:         kafkaSASLUser,
		SASLPassword:     kafkaSASLPassword,
		TLSEnabled:       kafkaTLSEnabled,
		TLSSkipVerify:    kafkaTLSSkipVerify,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KafkaTopic")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
