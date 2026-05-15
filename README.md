# Kafka Topic Controller

Kubernetes Controller untuk mengelola topik Kafka secara deklaratif melalui Custom Resource Definitions (CRD). Controller ini memungkinkan integrasi manajemen topik Kafka ke dalam alur kerja GitOps (seperti ArgoCD).

## Status Proyek

- **Code Coverage**: 15.6% (Target: 10%)

## Fitur

- **Multi-Cluster**: Kelola topik di berbagai cluster Kafka yang berbeda dalam satu controller.
- **Deklaratif**: Definisikan topik Kafka dalam format YAML.
- **Sinkronisasi Otomatis**: Menjamin *actual state* di Kafka sesuai dengan *desired state* di Kubernetes.
- **Manajemen Siklus Hidup**: Mendukung pembuatan, pembaruan konfigurasi/partisi, dan penghapusan topik.

## Persyaratan

- Cluster Kubernetes.
- Cluster Kafka yang dapat diakses dari cluster Kubernetes.
- `kubectl` dan `go` (untuk pengembangan).

## Instalasi

### 1. Register CRD
Daftarkan Custom Resource Definition langsung dari GitHub:
```bash
kubectl apply -f https://raw.githubusercontent.com/KAnggara75/kafka-topic-controller/main/config/crd/bases/kafka.kanggara.my.id_kafkatopics.yaml
```

### 2. Deploy Controller
Jalankan semua manifest deployment dalam satu perintah:
```bash
kubectl apply -f https://raw.githubusercontent.com/KAnggara75/kafka-topic-controller/main/deploy/install.yaml
```

> **Peringatan**: Perintah di atas menggunakan branch `main`. Pastikan file sudah dimerge ke branch `main` sebelum menjalankan perintah ini.

### 3. Jalankan Lokal (Development)
Jika ingin menjalankan secara lokal untuk pengembangan:
```bash
export KAFKA_BOOTSTRAP_SERVERS="localhost:9092"
go run main.go
```

Controller ini menggunakan `viper` untuk membaca konfigurasi. Anda dapat menggunakan environment variabel dengan prefix `KAFKA_` atau file konfigurasi.

### Konfigurasi Global (Default)
Digunakan sebagai fallback jika cluster spesifik tidak didefinisikan.

| Nama Variabel | Deskripsi |
|---------------|-----------|
| `KAFKA_BOOTSTRAP_SERVERS` | Alamat broker Kafka default. |
| `KAFKA_SECURITY_PROTOCOL` | Protokol keamanan (`PLAINTEXT`, `SASL_SSL`, dll). |
| `KAFKA_SASL_MECHANISM` | Mekanisme SASL (`PLAIN`, `SCRAM-SHA-256`, dll). |
| `KAFKA_SASL_USERNAME` | Username untuk SASL. |
| `KAFKA_SASL_PASSWORD` | Password untuk SASL. |
| `KAFKA_SSL_CA_LOCATION` | Lokasi sertifikat CA (bisa berupa URL atau path file). |

### Konfigurasi Multi-Cluster
Untuk mendukung multi-cluster, gunakan struktur berikut (contoh untuk cluster `localhost:9092`):
- `KAFKA_CLUSTERS_"localhost:9092"_SASL_USERNAME="user1"`
- `KAFKA_CLUSTERS_"localhost:9092"_SASL_PASSWORD="pass"`

Atau dalam file `config.yaml`:
```yaml
kafka:
  clusters:
    "localhost:9092":
      sasl:
        username: "user1"
        password: "pass"
      security:
        protocol: "SASL_SSL"
```

## Cara Penggunaan

Buat file manifest `kafkatopic.yaml`:

```yaml
apiVersion: kafka.kanggara.my.id/v1
kind: KafkaTopic
metadata:
  name: example-topic
spec:
  clusterUrl: "localhost:9092"
  partitions: 3
  replicationFactor: 1
  config:
    cleanup.policy: delete
    retention.ms: "604800000"
```

Aplikasikan ke Kubernetes:
```bash
kubectl apply -f kafkatopic.yaml
```

### Konfigurasi Spec

| Field | Tipe | Deskripsi |
|-------|------|-----------|
| `clusterUrl` | string | Alamat broker Kafka tujuan (misal: `localhost:9092`). |
| `partitions` | int | Jumlah partisi topik. |
| `replicationFactor` | int | Faktor replikasi topik. |
| `config` | map[string]string | Konfigurasi tambahan Kafka (misal: `cleanup.policy`). |

## Pengembangan

### Build Binary
```bash
make build
```

### Generate Manifests
Jika Anda melakukan perubahan pada struct API di `api/v1alpha1/`, jalankan:
```bash
make manifests
```

## Lisensi
Distributed under the MIT License. See `LICENSE` for more information.
