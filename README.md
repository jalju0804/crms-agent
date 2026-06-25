# VM Guest Metric Agent

Lightweight Go agent for collecting guest OS metrics inside an OpenStack VM.

## MVP Scope

- Reads OpenStack metadata from `http://169.254.169.254/openstack/latest/meta_data.json`.
- Collects CPU, memory, root filesystem, disk I/O, network I/O, and agent self metrics.
- Writes one JSON object per line to `/var/lib/vm-metric-agent/samples.jsonl`.
- Can write directly to Gnocchi for storage sizing experiments.
- Excludes noisy Kubernetes/container interfaces by prefix.
- Direct Gnocchi mode is for PoC/storage sizing only. Gateway integration is still the recommended operating model.

## Build

```sh
go test ./...
GOOS=linux GOARCH=amd64 go build -o build/vm-metric-agent-linux-amd64 ./cmd/vm-metric-agent
go build -o build/vm-metric-gateway ./cmd/vm-metric-gateway
```

## Run Gateway Locally

```sh
go run ./cmd/vm-metric-gateway -config configs/gateway-config.yaml
```

Gateway APIs:

- `GET /healthz`: process liveness.
- `GET /readyz`: Gnocchi archive policy readiness.
- `POST /v1/samples`: ingest one agent sample.
- `GET /v1/agents`: list known agents by most recent sample.
- `GET /v1/agents/{instance_id}`: get one agent status.

If `auth_token` is set in gateway config, agents must send
`Authorization: Bearer <token>`.

## Run Once Locally

```sh
go run ./cmd/vm-metric-agent -config configs/config.yaml -once
```

For local non-root testing, override `output_path` to a writable path.

## Agent Gateway Mode

Gateway mode sends samples to `vm-metric-gateway` instead of writing directly to
Gnocchi from each VM.

```yaml
interval: 60s
output_mode: gateway
output_path: /var/lib/vm-metric-agent/samples.jsonl
gateway:
  endpoint: http://127.0.0.1:8080
  token: ""
```

`output_path` remains required by the shared agent config, but gateway mode does
not append local JSONL samples. Keep `output_mode: jsonl` for local-only raw
samples, `output_mode: gnocchi` for direct PoC writes, and `output_mode: both`
for local JSONL plus direct Gnocchi writes.

## Documentation

- [VM Metric Gateway rollout summary](docs/vm-metric-gateway-rollout-summary.md): gateway design, deployed VM state, scale estimate, risks, and next steps.
- [Gnocchi storage sizing](docs/gnocchi-storage-sizing.md): retention policy, point count, and storage estimates.

## Install On Ubuntu VM

```sh
sudo ./install.sh
```

The installer creates:

- `/usr/local/bin/vm-metric-agent`
- `/etc/vm-metric-agent/config.yaml`
- `/var/lib/vm-metric-agent/samples.jsonl`
- `/etc/systemd/system/vm-metric-agent.service`
- `/etc/logrotate.d/vm-metric-agent`
- `vmmetric` system user

The logrotate policy keeps local raw JSONL for 3 daily rotations, compressing older files. Gnocchi remains the long-term store.

## Metrics

- `guest.cpu.used_percent`
- `guest.memory.used_percent`
- `guest.memory.available_bytes`
- `guest.filesystem.used_percent`
- `guest.disk.read_bytes`
- `guest.disk.write_bytes`
- `guest.net.rx_bytes`
- `guest.net.tx_bytes`
- `agent.loop_duration_ms`
- `agent.error_count`
- `agent.sample_size_bytes`
- `agent.rss_bytes`

## Gnocchi Storage Policy

The direct Gnocchi mode creates or reuses archive policy `vm_guest_usage_default`:

```yaml
aggregation_methods:
  - mean
  - min
  - max
definition:
  - granularity: 60s
    timespan: 7d
  - granularity: 5m
    timespan: 90d
  - granularity: 1h
    timespan: 365d
```

For a 60 second collection interval this is 44,760 retained points per metric:

- 60s for 7d: 10,080 points
- 5m for 90d: 25,920 points
- 1h for 365d: 8,760 points

With 10 metrics per VM, that is 447,600 retained points per VM.
# crms-agent
