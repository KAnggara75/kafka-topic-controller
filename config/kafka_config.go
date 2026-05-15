package config

import (
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/spf13/viper"
)

var (
	certCache sync.Map // map[string]string: URL -> local path
)

func getCertName(certURL string) string {
	u, err := url.Parse(certURL)
	if err != nil {
		return "kafka.cert"
	}
	base := filepath.Base(u.Path)
	if base == "." || base == "/" {
		base = "kafka"
	}
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Add a short hash of the URL to ensure uniqueness and re-download on URL change
	h := fnv.New32a()
	h.Write([]byte(certURL))
	hash := fmt.Sprintf("%x", h.Sum32())

	return fmt.Sprintf("%s-%s.cert", name, hash)
}

func ensureCert(certURL string) string {
	if path, ok := certCache.Load(certURL); ok {
		return path.(string)
	}

	const dir = "/tmp/kafka"
	certName := getCertName(certURL)
	localPath := filepath.Join(dir, certName)

	if err := os.MkdirAll(dir, 0750); err != nil {
		panic("Failed to create dir for Kafka cert: " + err.Error())
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		panic("Failed to open Kafka cert dir: " + err.Error())
	}
	defer root.Close()

	if _, err := root.Stat(certName); os.IsNotExist(err) {
		resp, err := http.Get(certURL) //#nosec G107
		if err != nil {
			panic("Failed to download Kafka cert: " + err.Error())
		}
		defer resp.Body.Close()

		out, err := root.Create(certName)
		if err != nil {
			panic("Failed to create Kafka cert file: " + err.Error())
		}
		defer out.Close()

		if _, err := io.Copy(out, resp.Body); err != nil {
			panic("Failed to write Kafka cert file: " + err.Error())
		}
	}

	certCache.Store(certURL, localPath)
	return localPath
}

func GetBaseKafkaConfig(clusterName string) *ckafka.ConfigMap {
	// Helper to get config with fallback to global kafka.*
	getConfig := func(key string) string {
		// 1. Try kafka.<clusterName>.<key>
		fullPath := fmt.Sprintf("kafka.%s.%s", clusterName, key)
		if val := viper.GetString(fullPath); val != "" {
			return val
		}

		// 2. Fallback to global kafka.<key>
		return viper.GetString("kafka." + key)
	}

	bootstrapServers := getConfig("bootstrap.servers")
	if bootstrapServers == "" {
		// Fallback to using clusterName as bootstrap server if not defined in config
		bootstrapServers = clusterName
	}

	certLocation := getConfig("ssl.ca.location")

	if certLocation != "" {
		// cek apakah certLocation adalah URL
		if u, err := url.Parse(certLocation); err == nil && (strings.HasPrefix(u.Scheme, "http")) {
			certLocation = ensureCert(certLocation)
		} else {
			// pastikan file ada
			if _, err := os.Stat(certLocation); os.IsNotExist(err) {
				panic("Kafka cert file not found: " + certLocation)
			}
		}
	}

	configMap := &ckafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"security.protocol": getConfig("security.protocol"),
		"sasl.mechanism":    getConfig("sasl.mechanism"),
		"sasl.username":     getConfig("sasl.username"),
		"sasl.password":     getConfig("sasl.password"),
	}

	if certLocation != "" {
		fmt.Printf("[DEBUG] Using SSL CA location for %s: %s\n", clusterName, certLocation)
		(*configMap)["ssl.ca.location"] = certLocation
	} else {
		fmt.Printf("[DEBUG] No SSL CA location found for %s, using system defaults\n", clusterName)
	}

	return configMap
}
