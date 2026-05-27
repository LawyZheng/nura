# Nura / 知愈

A local-first health AI agent focused on duodenal ulcer management. Nura helps patients track symptoms, diet, and medications, ingest and interpret medical reports, and get AI-powered health insights — all while keeping data on the user's own device.

## Health Safety Disclaimer

**Nura is NOT a medical device.** It does not diagnose, prescribe, or replace professional medical advice. All AI-generated content is for health reference only. Always consult a qualified healthcare professional for medical decisions.

- Emergency symptoms (black stool, hematemesis, severe pain) trigger immediate "seek medical attention" responses
- All outputs include safety disclaimers and confidence indicators
- Medication-related queries require guideline-backed sources

## Quick Start

```bash
# Build
go build -o nura ./cmd/nura

# Start the server
./nura serve --port 8000

# Ingest a sample report
./nura ingest data/sample_gastroscopy_report.txt

# Send a query
curl -X POST http://localhost:8000/agent/run \
  -H "Content-Type: application/json" \
  -d '{"user_message": "什么是 DOB 值？"}'

# Run tests
go test ./...
```

## Architecture

```
App / CLI
  └── HTTP API (Gin — Agent Gateway)
        └── Health Agent Runtime
              ├── Planner / Risk Classifier
              ├── Context Builder (Patient Memory)
              ├── Tool Runtime (Report Ingestion, etc.)
              ├── Policy / Safety Engine
              └── LLM Provider (pluggable)
                    └── SQLite (GORM + modernc, pure Go)
```

**Design Principles:**
- **Agent-first**: Business logic lives in the Health Agent Runtime, not in HTTP handlers
- **Local-first**: All data stored in local SQLite — no cloud uploads by default
- **Memory-driven**: Context Builder dynamically assembles patient context per task
- **Safety-first**: Policy Engine enforces risk classification, disclaimers, and source citations
- **Traceable**: Every agent run produces a full execution trace

## Documentation

- [CLI Reference](docs/cli.md) — all commands, flags, config, environment variables
- [HTTP API Reference](docs/api.md) — endpoints, request/response schemas, examples
- Swagger UI: available at `/swagger/index.html` when the server is running

## Library Choices

| Library | Purpose | Rationale |
|---------|---------|-----------|
| [Gin](https://github.com/gin-gonic/gin) | HTTP router | Fast, middleware-rich, battle-tested |
| [Cobra](https://github.com/spf13/cobra) | CLI framework | Standard for Go CLIs (kubectl, docker, hugo) |
| [Viper](https://github.com/spf13/viper) | Configuration | YAML/env/flag unification, works with Cobra |
| [GORM](https://gorm.io) + [glebarez/sqlite](https://github.com/glebarez/sqlite) | ORM + SQLite | Type-safe queries, auto-migration, pure Go (no CGO) |
| [Zap](https://go.uber.org/zap) | Structured logging | High-performance, JSON/console output |
| [Swaggo](https://github.com/swaggo/swag) | API documentation | Auto-generates OpenAPI from handler annotations |

## Project Structure

```
cmd/nura/              CLI entry point (Cobra commands: serve, ingest, trace, version)
configs/               Example configuration files
docs/                  CLI and API documentation
internal/
  config/              Configuration loading (Viper)
  logging/             Structured logging (Zap)
  runtime/             Agent runtime, tool registry, tracing
  tools/               Tool interface and implementations
  memory/              Patient memory (raw events, structured state, context builder)
  policy/              Risk classifier, medical policy, output guard
  pipeline/            Report ingestion pipeline
  providers/           LLM provider interface and mock
  gateway/             HTTP server (Gin)
  store/               SQLite storage layer (GORM)
  model/               Domain models (GORM-tagged)
knowledge/             Medical knowledge base (medications, diet, indicators)
proto/                 gRPC proto for Phase 2+ Python sidecar
data/                  Synthetic report samples for testing
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/agent/run` | Run agent with user message |
| POST | `/agent/report/ingest` | Ingest a medical report |
| GET | `/agent/debug/trace/:trace_id` | View execution trace |
| GET | `/swagger/index.html` | Swagger API documentation |

## Configuration

Nura supports configuration via YAML file, environment variables, and CLI flags. See [CLI docs](docs/cli.md) for details.

```bash
# Using config file
./nura serve --config config.yaml

# Using environment variables
NURA_SERVER_PORT=9000 NURA_LOGGING_LEVEL=debug ./nura serve
```

## Key Design Decisions

- **No third-party agent frameworks**: Uses Nura-owned interfaces + official LLM SDKs
- **Pure Go SQLite**: `glebarez/sqlite` (wraps modernc.org/sqlite) — no CGO dependency
- **Mock LLM for testing**: All tests pass without real API keys
- **Single binary distribution**: No runtime dependencies

## License

See [LICENSE](LICENSE).
