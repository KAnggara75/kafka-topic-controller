# Kafka Topic Controller

Kubernetes Controller untuk mengelola topik Kafka secara deklaratif melalui Custom Resource Definitions (CRD). Controller ini memungkinkan integrasi manajemen topik Kafka ke dalam alur kerja GitOps (seperti ArgoCD).

## Status Proyek

- **Code Coverage**: 16.7% (Target: 10%)

## Fitur

- **Deklaratif**: Definisikan topik Kafka dalam format YAML.
- **Sinkronisasi Otomatis**: Menjamin *actual state* di Kafka sesuai dengan *desired state* di Kubernetes.
- **Idempotent**: Menggunakan `topicctl` untuk melakukan perubahan yang aman dan efisien.
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

### Konfigurasi Kafka

Controller ini menggunakan variabel lingkungan berikut untuk koneksi ke Kafka:

| Nama Variabel | Deskripsi |
|---------------|-----------|
| `KAFKA_BOOTSTRAP_SERVERS` | Alamat broker Kafka (misal: `kafka.example.com:9092`). |
| `SASL_MECHANISM` | Mekanisme SASL (`PLAIN`, `SCRAM-SHA-256`, dll). |
| `SECURITY_PROTOCOL` | Set ke `SASL_SSL` untuk mengaktifkan TLS secara otomatis. |
| `SASL_JAAS_CONFIG` | String JAAS untuk autentikasi. |
| `SSL_TRUSTSTORE_LOCATION` | Lokasi file `.jks` atau PEM untuk verifikasi TLS. |
| `SSL_TRUSTSTORE_PASSWORD` | Password untuk file JKS. |

> **Catatan**: Jika `SSL_TRUSTSTORE_LOCATION` menggunakan file `.jks`, controller akan secara otomatis melakukan konversi ke format PEM di memori.

## Cara Penggunaan

Buat file manifest `kafkatopic.yaml`:

```yaml
apiVersion: kafka.kanggara.my.id/v1alpha1
kind: KafkaTopic
metadata:
  name: example-topic
spec:
  partitions: 3
  replicationFactor: 2
  retentionMinutes: 1440
  settings:
    cleanup.policy: delete
    min.insync.replicas: "2"
    retention.bytes: "1073741824"
```

Aplikasikan ke Kubernetes:
```bash
kubectl apply -f kafkatopic.yaml
```

### Konfigurasi Spec

| Field | Tipe | Deskripsi |
|-------|------|-----------|
| `partitions` | int | Jumlah partisi topik. |
| `replicationFactor` | int | Faktor replikasi topik. |
| `retentionMinutes` | int | Waktu retensi dalam menit. |
| `settings` | map[string]string | Konfigurasi tambahan Kafka (misal: `cleanup.policy`). |

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
