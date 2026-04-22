# -------- BUILD STAGE --------
FROM golang:1.22-bookworm AS builder

WORKDIR /build

# Install dependency untuk CGO + Kafka
RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    build-essential \
    librdkafka-dev \
 && rm -rf /var/lib/apt/lists/*

# Cache dependency
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary (CGO wajib aktif)
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -trimpath \
    -o kafka-topic-controller ./main.go


# -------- RUNTIME STAGE --------
FROM debian:bookworm-slim

# Install runtime dependency (WAJIB untuk confluent)
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    librdkafka1 \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/kafka-topic-controller .

# Optional: security hardening
RUN useradd -u 10001 appuser
USER appuser

ENV PORT=8081
ENV TZ=Asia/Jakarta

EXPOSE 8081

CMD ["./kafka-topic-controller"]
