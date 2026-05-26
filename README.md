# Nura / 知愈

A local-first health AI agent focused on duodenal ulcer management. Nura helps patients track symptoms, diet, and medications, ingest and interpret medical reports, and get AI-powered health insights — all while keeping data on the user's own device.

## Health Safety Disclaimer

**Nura is NOT a medical device.** It does not diagnose, prescribe, or replace professional medical advice. All AI-generated content is for health reference only. Always consult a qualified healthcare professional for medical decisions.

- Emergency symptoms (black stool, hematemesis, severe pain) trigger immediate "seek medical attention" responses
- All outputs include safety disclaimers and confidence indicators
- Medication-related queries require guideline-backed sources

## Architecture

```
App / CLI
  └── HTTP API (Agent Gateway)
        └── Health Agent Runtime
              ├── Planner / Risk Classifier
              ├── Context Builder (Patient Memory)
              ├── Tool Runtime (Report Ingestion, etc.)
              ├── Policy / Safety Engine
              └── LLM Provider (pluggable)
```

- **Agent-first**: Business logic lives in the Health Agent Runtime, not in HTTP handlers
- **Local-first**: All data stored in local SQLite (via `modernc.org/sqlite`, pure Go, no CGO)
- **Memory-driven**: Context Builder dynamically assembles patient context per task
- **Safety-first**: Policy Engine enforces risk classification, disclaimers, and source citations
- **Traceable**: Every agent run produces a full execution trace

## Build & Run

```bash
# Build
go build ./cmd/nura

# Run HTTP server
./nura serve --port 8000 --data-dir ~/.nura

# Ingest a report from file
./nura ingest data/sample_gastroscopy_report.txt

# Run tests
go test ./...
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/agent/run` | Run agent with user message |
| POST | `/agent/report/ingest` | Ingest a medical report |
| GET | `/agent/debug/trace/{id}` | View execution trace |
| GET | `/health` | Health check |

## Project Structure

```
cmd/nura/              CLI entry point (serve, ingest, trace)
internal/
  runtime/             Agent runtime, tool registry, tracing
  tools/               Tool interface and implementations
  memory/              Patient memory (raw events, structured state, context builder)
  policy/              Risk classifier, medical policy, output guard
  pipeline/            Report ingestion pipeline
  providers/           LLM provider interface and mock
  gateway/             HTTP server
  store/               SQLite storage layer
  model/               Domain models
knowledge/             Medical knowledge base (medications, diet, indicators)
proto/                 gRPC proto for Phase 2+ Python sidecar
data/                  Synthetic report samples for testing
```

## Key Design Decisions

- **No third-party agent frameworks**: Uses Nura-owned interfaces + official LLM SDKs
- **Pure Go SQLite**: `modernc.org/sqlite` — no CGO dependency
- **Mock LLM for testing**: All tests pass without real API keys
- **Single binary distribution**: ~15-20MB, no runtime dependencies

## License

See [LICENSE](LICENSE).
