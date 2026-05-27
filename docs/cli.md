# Nura CLI Reference

## Installation

```bash
# From source
go install github.com/LawyZheng/nura/cmd/nura@latest

# Or build locally
git clone https://github.com/LawyZheng/nura.git
cd nura
go build -o nura ./cmd/nura

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o nura-linux ./cmd/nura
GOOS=darwin GOARCH=arm64 go build -o nura-darwin ./cmd/nura
```

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `""` | Path to config file (YAML) |

## Commands

### `nura serve`

Start the HTTP server (Agent Gateway).

```bash
nura serve [flags]
```

**Flags:**

| Flag | Default | Env Var | Description |
|------|---------|---------|-------------|
| `--port` | `8000` | `NURA_SERVER_PORT` | Server port |
| `--data-dir` | `~/.nura` | `NURA_SERVER_DATADIR` | Data directory for SQLite and files |

**Examples:**

```bash
# Start with defaults
nura serve

# Custom port and data directory
nura serve --port 9000 --data-dir /path/to/data

# With config file
nura serve --config ~/.nura/config.yaml

# Using environment variables
NURA_SERVER_PORT=9000 nura serve
```

### `nura ingest <file>`

Ingest a medical report from a text file. Runs the report through the ingestion pipeline (classify → extract → normalize → merge → explain) and outputs the structured result as JSON.

```bash
nura ingest <file>
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `file` | Yes | Path to the report text file |

**Examples:**

```bash
# Ingest a gastroscopy report
nura ingest data/sample_gastroscopy_report.txt

# Ingest and save output
nura ingest report.txt > result.json

# Pipe with jq for specific fields
nura ingest report.txt | jq '.extracted_facts'
```

### `nura trace <id>`

View an agent execution trace. Currently requires the server to be running — prints instructions for using the HTTP API.

```bash
nura trace <id>
```

**Examples:**

```bash
# View a specific trace
nura trace abc123-def456

# Or query the running server directly
curl http://localhost:8000/agent/debug/trace/abc123-def456
```

### `nura version`

Print version information.

```bash
nura version
```

## Configuration File

Nura uses YAML configuration. See `configs/config.example.yaml` for all options.

**Default config location:** `~/.nura/config.yaml`

```yaml
server:
  port: 8000
  data_dir: ~/.nura

database:
  path: nura.db

llm:
  provider: mock    # mock / anthropic / openai / qwen
  api_key: ""
  model: ""
  temperature: 0.1

logging:
  level: info       # debug / info / warn / error
  format: console   # console / json
```

**Priority order:** CLI flags > Environment variables > Config file > Defaults

## Environment Variables

All config options can be set via environment variables with the `NURA_` prefix:

| Variable | Config Key | Description |
|----------|------------|-------------|
| `NURA_SERVER_PORT` | `server.port` | Server port |
| `NURA_SERVER_DATADIR` | `server.data_dir` | Data directory |
| `NURA_DATABASE_PATH` | `database.path` | Database filename |
| `NURA_LLM_PROVIDER` | `llm.provider` | LLM provider name |
| `NURA_LLM_APIKEY` | `llm.api_key` | LLM API key |
| `NURA_LLM_MODEL` | `llm.model` | LLM model name |
| `NURA_LLM_TEMPERATURE` | `llm.temperature` | LLM temperature |
| `NURA_LOGGING_LEVEL` | `logging.level` | Log level |
| `NURA_LOGGING_FORMAT` | `logging.format` | Log format |
