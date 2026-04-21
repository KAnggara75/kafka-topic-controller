package main

import (
	"flag"
	"os"
	"regexp"
	"strings"

	"encoding/pem"

	"github.com/pavlo-v-chernykh/keystore-go/v4"

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

func getEnv(keys ...string) string {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return ""
}

func parseJAASConfig(jaas string) (string, string) {
	usernameRegex := regexp.MustCompile(`username="([^"]+)"`)
	passwordRegex := regexp.MustCompile(`password="([^"]+)"`)

	usernameMatch := usernameRegex.FindStringSubmatch(jaas)
	passwordMatch := passwordRegex.FindStringSubmatch(jaas)

	var user, pass string
	if len(usernameMatch) > 1 {
		user = usernameMatch[1]
	}
	if len(passwordMatch) > 1 {
		pass = passwordMatch[1]
	}
	return user, pass
}

func convertJKStoPEM(jksPath, password string) (string, error) {
	f, err := os.Open(jksPath) // #nosec G304
	if err != nil {
		return "", err
	}
	defer f.Close()

	ks := keystore.New()
	if err := ks.Load(f, []byte(password)); err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp("", "kafka-ca-*.pem")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	for _, alias := range ks.Aliases() {
		if ks.IsTrustedCertificateEntry(alias) {
			entry, err := ks.GetTrustedCertificateEntry(alias)
			if err != nil {
				continue
			}
			if err := pem.Encode(tempFile, &pem.Block{
				Type:  "CERTIFICATE",
				Bytes: entry.Certificate.Content,
			}); err != nil {
				return "", err
			}
		} else if ks.IsPrivateKeyEntry(alias) {
			entry, err := ks.GetPrivateKeyEntry(alias, []byte(password))
			if err != nil {
				continue
			}
			for _, cert := range entry.CertificateChain {
				if err := pem.Encode(tempFile, &pem.Block{
					Type:  "CERTIFICATE",
					Bytes: cert.Content,
				}); err != nil {
					return "", err
				}
			}
		}
	}

	return tempFile.Name(), nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var kafkaBootstrapServers string

	var kafkaSASLMechanism string
	var kafkaTLSEnabled bool
	var kafkaTLSSkipVerify bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	// Support both standard and user-specific env var names
	bootstrap := getEnv("KAFKA_BOOTSTRAP_SERVERS", "KAFKA_BOOTSTRAPSERVERS", "BOOTSTRAP_SERVER")
	mechanism := getEnv("KAFKA_SASL_MECHANISM", "KAFKA_PROPERTIES_SASL_MECHANISM", "SASL_MECHANISM")
	securityProtocol := getEnv("KAFKA_SECURITY_PROTOCOL", "KAFKA_PROPERTIES_SECURITY_PROTOCOL", "SECURITY_PROTOCOL")
	jaasConfig := getEnv("KAFKA_SASL_JAAS_CONFIG", "KAFKA_PROPERTIES_SASL_JAAS_CONFIG", "SASL_JAAS_CONFIG")
	tlsEnabledEnv := getEnv("KAFKA_TLS_ENABLED", "KAFKA_PROPERTIES_TLS_ENABLED")
	tlsSkipVerifyEnv := getEnv("KAFKA_TLS_SKIP_VERIFY", "KAFKA_PROPERTIES_TLS_SKIP_VERIFY")
	caCertPath := getEnv("KAFKA_CA_CERT_PATH", "SSL_TRUSTSTORE_LOCATION")
	caCertPassword := getEnv("SSL_TRUSTSTORE_PASSWORD", "KAFKA_CA_CERT_PASSWORD")

	if caCertPath != "" && strings.HasSuffix(caCertPath, ".jks") {
		setupLog.Info("Converting JKS truststore to PEM", "path", caCertPath)
		pemPath, err := convertJKStoPEM(caCertPath, caCertPassword)
		if err != nil {
			setupLog.Error(err, "Failed to convert JKS to PEM")
		} else {
			caCertPath = pemPath
			setupLog.Info("JKS converted to PEM", "pemPath", pemPath)
		}
	}

	// Special handling for SASL_SSL protocol
	tlsEnabled := tlsEnabledEnv == "true" || strings.Contains(securityProtocol, "SSL")

	kafkaSASLUser, kafkaSASLPassword := parseJAASConfig(jaasConfig)
	if kafkaSASLUser == "" {
		kafkaSASLUser = os.Getenv("KAFKA_SASL_USER")
	}
	if kafkaSASLPassword == "" {
		kafkaSASLPassword = os.Getenv("KAFKA_SASL_PASSWORD")
	}

	flag.StringVar(&kafkaBootstrapServers, "kafka-bootstrap-servers", bootstrap, "Kafka bootstrap servers")
	flag.StringVar(&kafkaSASLMechanism, "kafka-sasl-mechanism", mechanism, "Kafka SASL mechanism")
	flag.StringVar(&kafkaSASLUser, "kafka-sasl-user", kafkaSASLUser, "Kafka SASL user")
	flag.StringVar(&kafkaSASLPassword, "kafka-sasl-password", kafkaSASLPassword, "Kafka SASL password")
	flag.BoolVar(&kafkaTLSEnabled, "kafka-tls-enabled", tlsEnabled, "Enable Kafka TLS")
	flag.BoolVar(&kafkaTLSSkipVerify, "kafka-tls-skip-verify", tlsSkipVerifyEnv == "true", "Skip Kafka TLS verification")

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
		TLSCACertPath:    caCertPath,
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
