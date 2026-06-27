# ShardRoute

A distributed rate-limiting and request-routing service written in Go, using a hybrid token-bucket + sliding-window algorithm, atomic Redis Lua scripting, a local in-memory caching layer to avoid per-request network round-trips, and explicit fail-open/fail-closed degradation when Redis is unreachable.

## Architecture

```text
[ Clients ] ---> [ Load Balancer ]
                        |
            +-----------+-----------+
            |                       |
    [ ShardRoute 1 ]        [ ShardRoute 2 ]
    (Local Cache)           (Local Cache)
            |                       |
            +-----------+-----------+
                        |
                 [ Redis Server ]
             (Authoritative State)
```

## Quick Start

1. Start the multi-node simulation (3 ShardRoute nodes + 1 Redis instance):
   ```bash
   cd deploy
   docker-compose up -d
   ```

2. Send a request to a node:
   ```bash
   curl -X POST http://localhost:8081/v1/check \
     -H "Content-Type: application/json" \
     -d '{"key": "user_123", "cost": 1, "limit_name": "api"}'
   ```

## Production vs Design

- **Live Deployment:** Lua-based atomicity and fail-open/closed behavior are validated against a live Redis instance.
- **Cluster Support:** True multi-shard Redis Cluster behavior is implemented and unit-tested via the `go-redis` cluster client but not yet deployed against a live cluster.
- **Local Cache:** A highly performant in-memory cache syncs periodically with Redis, bounded by `SyncIntervalMS`.

## Load Testing

The benchmark proves the effectiveness of the local caching layer. See `loadtest/` for k6 scripts simulating 10,000 concurrent users. 

**Metrics**:
You can track real-time results via Prometheus on `/metrics` and the provided Grafana dashboard in `deploy/grafana/dashboard.json`.

## Local Development

Requirements:
- Go 1.25+
- Redis (or Docker)
- Protobuf Compiler (`protoc`)

Build the CLI bench tool and server:
```bash
make build
```

Run tests:
```bash
make test
```
