package config

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/spf13/viper"
)

var (
	once          sync.Once
	localCertPath string
)

func downloadCertOnce(certURL string) string {
	once.Do(func() {
		const (
			dir      = "/tmp/kafka"
			certName = "kafka.cert"
		)

		if err := os.MkdirAll(dir, 0750); err != nil {
			panic("Failed to create dir for Kafka cert: " + err.Error())
		}

		// Use os.Root to scope all file operations under dir,
		// preventing any directory-traversal (CWE-22 / G304).
		root, err := os.OpenRoot(dir)
		if err != nil {
			panic("Failed to open Kafka cert dir: " + err.Error())
		}
		defer func() {
			if err := root.Close(); err != nil {
				panic("Failed to close Kafka cert root: " + err.Error())
			}
		}()

		// Absolute path used only for passing to the Kafka client later.
		localCertPath = filepath.Join(dir, certName)

		// Skip download if the file is already present.
		if _, err := root.Stat(certName); os.IsNotExist(err) {
			resp, err := http.Get(certURL) //#nosec G107 -- URL is sourced from trusted application config, not user input
			if err != nil {
				panic("Failed to download Kafka cert: " + err.Error())
			}
			defer func(Body io.ReadCloser) {
				if err := Body.Close(); err != nil {
					panic("Failed to close Kafka cert response body: " + err.Error())
				}
			}(resp.Body)

			// root.Create is scoped to dir — no traversal possible.
			out, err := root.Create(certName)
			if err != nil {
				panic("Failed to create Kafka cert file: " + err.Error())
			}
			defer func(out *os.File) {
				if err := out.Close(); err != nil {
					panic("Failed to close Kafka cert file: " + err.Error())
				}
			}(out)

			if _, err := io.Copy(out, resp.Body); err != nil {
				panic("Failed to write Kafka cert file: " + err.Error())
			}
		}
	})
	return localCertPath
}

func GetBaseKafkaConfig() *kafka.ConfigMap {
	certLocation := viper.GetString("kafka.ssl.ca.location")

	// cek apakah certLocation adalah URL
	if u, err := url.Parse(certLocation); err == nil && (strings.HasPrefix(u.Scheme, "http")) {
		certLocation = downloadCertOnce(certLocation)
	} else {
		// pastikan file ada
		if _, err := os.Stat(certLocation); os.IsNotExist(err) {
			panic("Kafka cert file not found: " + certLocation)
		}
	}

	return &kafka.ConfigMap{
		"bootstrap.servers": viper.GetString("kafka.bootstrap.servers"),
		"security.protocol": viper.GetString("kafka.security.protocol"),
		"ssl.ca.location":   certLocation,
		"sasl.mechanism":    viper.GetString("kafka.sasl.mechanism"),
		"sasl.username":     viper.GetString("kafka.sasl.username"),
		"sasl.password":     viper.GetString("kafka.sasl.password"),
	}
}
