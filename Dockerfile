FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/ws ./cmd/ws
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

# Needs a shell: carpenter runs a registry service's run_command as
# `/bin/sh -c <cmd>`, which is how /api, /ws and /worker are selected from this
# one image. A distroless base has no shell and cannot start those services.
FROM alpine:3.22

COPY --from=builder /bin/api /bin/ws /bin/worker /

USER 1001

ENTRYPOINT ["/api"]
