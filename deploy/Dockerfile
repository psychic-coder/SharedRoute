FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /shardroute ./cmd/shardroute

FROM gcr.io/distroless/static-debian12:latest

COPY --from=builder /shardroute /usr/local/bin/shardroute
COPY --from=builder /app/config.example.yaml /config.yaml

EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/shardroute"]
