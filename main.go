package main

import (
	"encoding/pem"
	"flag"
	"os"
	"regexp"
	"strings"

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
	f, err := os.Open(jksPath) // #nosec G304 G703
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Check for JKS magic bytes
	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return "", err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	if magic[0] != 0xFE || magic[1] != 0xED || magic[2] != 0xFE || magic[3] != 0xED {
		setupLog.Info("Warning: File does not have JKS magic bytes (0xFEEDFEED)", "magic", magic)
	}

	ks := keystore.New()
	if err := ks.Load(f, []byte(password)); err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp("", "kafka-ca-*.pem")
	if err != nil {
		return "", err
	}
	// We keep the file open while encoding, then return the name.
	// The caller is responsible for deleting it if needed, though here it stays for the life of the process.

	for _, alias := range ks.Aliases() {
		if ks.IsTrustedCertificateEntry(alias) {
			setupLog.Info("Extracting trusted certificate", "alias", alias)
			entry, err := ks.GetTrustedCertificateEntry(alias)
			if err != nil {
				setupLog.Error(err, "Failed to get trusted cert entry", "alias", alias)
				continue
			}
			if err := pem.Encode(tempFile, &pem.Block{
				Type:  "CERTIFICATE",
				Bytes: entry.Certificate.Content,
			}); err != nil {
				_ = tempFile.Close()
				return "", err
			}
		} else if ks.IsPrivateKeyEntry(alias) {
			setupLog.Info("Extracting certificate chain from private key", "alias", alias)
			entry, err := ks.GetPrivateKeyEntry(alias, []byte(password))
			if err != nil {
				setupLog.Error(err, "Failed to get private key entry", "alias", alias)
				continue
			}
			for i, cert := range entry.CertificateChain {
				setupLog.Info("Encoding cert from chain", "alias", alias, "index", i)
				if err := pem.Encode(tempFile, &pem.Block{
					Type:  "CERTIFICATE",
					Bytes: cert.Content,
				}); err != nil {
					_ = tempFile.Close()
					return "", err
				}
			}
		}
	}

	_ = tempFile.Close()
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

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Support both standard and user-specific env var names
	bootstrap := os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	mechanism := os.Getenv("SASL_MECHANISM")
	securityProtocol := os.Getenv("SECURITY_PROTOCOL")
	jaasConfig := os.Getenv("SASL_JAAS_CONFIG")
	tlsEnabledEnv := os.Getenv("KAFKA_TLS_ENABLED")
	tlsSkipVerifyEnv := os.Getenv("KAFKA_TLS_SKIP_VERIFY")
	caCertPath := strings.TrimSpace(os.Getenv("SSL_TRUSTSTORE_LOCATION"))
	caCertPassword := strings.TrimSpace(os.Getenv("SSL_TRUSTSTORE_PASSWORD"))

	if caCertPath != "" && strings.HasSuffix(caCertPath, ".jks") {
		setupLog.Info("Converting JKS truststore to PEM", "path", caCertPath)
		pemPath, err := convertJKStoPEM(caCertPath, caCertPassword)
		if err != nil {
			setupLog.Error(err, "CRITICAL: Failed to convert JKS to PEM. Connection will likely fail.")
			os.Exit(1)
		} else {
			caCertPath = pemPath
			setupLog.Info("JKS converted to PEM successfully", "pemPath", pemPath)
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
