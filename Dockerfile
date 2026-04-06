FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api & \
    CGO_ENABLED=0 GOOS=linux go build -o /bin/ws ./cmd/ws & \
    CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker & \
    wait

FROM mongo:7

RUN apt-get update && apt-get install -y --no-install-recommends \
    redis-server redis-tools \
  && rm -rf /var/lib/apt/lists/*

RUN useradd -u 1001 -r -m miab \
  && mkdir -p /data/db /var/log/miab \
  && chown -R miab:miab /data/db /var/log/miab

COPY --from=builder /bin/api /bin/ws /bin/worker /
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER miab

ENTRYPOINT ["/entrypoint.sh"]
