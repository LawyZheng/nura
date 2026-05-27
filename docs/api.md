# Nura HTTP API Reference

Base URL: `http://localhost:8000` (default)

## Authentication

Currently local-only. No authentication required. TBD for future remote access.

## Endpoints

### Health Check

```
GET /health
```

Returns server health status.

**Response:**

```json
{
  "status": "ok"
}
```

**Example:**

```bash
curl http://localhost:8000/health
```

---

### Agent Run

```
POST /agent/run
```

Send a message to the health agent. The agent classifies risk, enforces safety policies, builds patient context, calls the LLM, and returns a guarded response.

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user_message` | string | Yes | User's question or task |
| `task_hint` | string | No | Hint for context building: `chat`, `report_ingest`, `visit_summary` |
| `patient_id` | string | No | Patient identifier (default: `local-default`) |

**Response:**

| Field | Type | Description |
|-------|------|-------------|
| `user_facing_reply` | string | The agent's response (with safety disclaimers) |
| `safety_labels` | object | Risk level, disclaimer status, source availability |
| `trace_id` | string | UUID for retrieving the full execution trace |
| `memory_updates` | string[] | Any patient memory updates performed |
| `result` | object | Additional structured result data |

**Safety Labels:**

| Field | Type | Description |
|-------|------|-------------|
| `risk_level` | string | `low` / `medium` / `high` / `emergency` |
| `has_disclaimer` | boolean | Whether a medical disclaimer was added |
| `has_sources` | boolean | Whether source citations were included |
| `has_confidence` | boolean | Whether a confidence level was assigned |
| `is_emergency` | boolean | Whether emergency keywords were detected |

**Examples:**

```bash
# Simple question
curl -X POST http://localhost:8000/agent/run \
  -H "Content-Type: application/json" \
  -d '{"user_message": "什么是 DOB 值？"}'

# With task hint
curl -X POST http://localhost:8000/agent/run \
  -H "Content-Type: application/json" \
  -d '{"user_message": "最近症状有好转吗", "task_hint": "chat"}'

# Emergency detection
curl -X POST http://localhost:8000/agent/run \
  -H "Content-Type: application/json" \
  -d '{"user_message": "我拉了黑便怎么办"}'
```

**Emergency Response:** When emergency keywords are detected (black stool, hematemesis, severe pain, etc.), the agent returns immediately with an emergency message and `is_emergency: true`. Normal processing is skipped.

---

### Report Ingestion

```
POST /agent/report/ingest
```

Ingest a medical report through the 5-stage pipeline: classify → extract → normalize → merge → explain.

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `raw_text` | string | Yes | Raw text content of the medical report |
| `report_date` | string | No | Date of the report (YYYY-MM-DD) |
| `source_type` | string | No | `photo` or `pdf` |
| `patient_id` | string | No | Patient identifier |

**Response:**

| Field | Type | Description |
|-------|------|-------------|
| `report_type` | string | Classified type (gastroscopy, hp_breath, blood_routine, etc.) |
| `extracted_facts` | object | Structured facts extracted by LLM |
| `normalized_indicators` | array | Standardized medical indicators |
| `merge_actions` | string[] | Actions taken to merge with existing data |
| `patient_state_updates` | string[] | Patient state changes |
| `missing_or_uncertain_fields` | string[] | Fields that couldn't be reliably extracted |
| `user_facing_explanation` | string | Patient-friendly explanation in Chinese |
| `stages` | array | Pipeline stage results for debugging |

**Supported Report Types:**

| Type | Description |
|------|-------------|
| `gastroscopy` | Gastroscopy / endoscopy report |
| `hp_breath` | C13/C14 H. pylori breath test |
| `hp_antibody` | H. pylori antibody test |
| `blood_routine` | Complete blood count |
| `liver_function` | Liver function panel |
| `kidney_function` | Kidney function panel |
| `stool_routine` | Stool routine test |
| `stool_occult_blood` | Fecal occult blood test |
| `general_checkup` | Comprehensive health checkup |
| `unknown` | Unclassified report |

**Example:**

```bash
curl -X POST http://localhost:8000/agent/report/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "raw_text": "胃镜检查报告\n十二指肠球部：前壁可见一处溃疡...",
    "report_date": "2024-03-01"
  }'
```

---

### Debug Trace

```
GET /agent/debug/trace/:trace_id
```

Retrieve the full execution trace for an agent run. Useful for debugging and understanding how the agent processed a request.

**Path Parameters:**

| Parameter | Description |
|-----------|-------------|
| `trace_id` | The trace UUID returned by `/agent/run` or `/agent/report/ingest` |

**Response:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Trace UUID |
| `started_at` | string | ISO 8601 timestamp |
| `finished_at` | string | ISO 8601 timestamp |
| `input` | string | Original input |
| `output` | string | Final output |
| `steps` | array | Ordered execution steps |
| `duration_ms` | integer | Total duration in milliseconds |

**Step Types:**

| Type | Description |
|------|-------------|
| `policy_check` | Risk classification or policy enforcement |
| `plan` | Context building or task planning |
| `llm_call` | LLM completion call |
| `tool_call` | Tool execution |
| `guard` | Output guard / safety check |

**Example:**

```bash
curl http://localhost:8000/agent/debug/trace/abc123-def456-789
```

---

## Error Responses

All error responses follow this format:

```json
{
  "error": "description of the error"
}
```

**HTTP Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Bad request (missing required fields, invalid input) |
| 404 | Resource not found (trace ID) |
| 405 | Method not allowed |
| 500 | Internal server error |

## Rate Limiting

TBD. Currently no rate limiting is applied (local-first, single-user design).

## CORS

CORS is enabled for all origins in development mode to support local Flutter/web clients.
