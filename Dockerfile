FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/ws ./cmd/ws
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/api /bin/ws /bin/worker /

USER 1001

ENTRYPOINT ["/api"]
