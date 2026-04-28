# study-agent

**Note**: This was built using agentic AI as a personal project to help me do some exam prep. As such, I would recommend setting limits on any AI keys you setup for this application as a precaution!

An AI-powered study assistant written in Go. It ingests notes, extracts structured knowledge, generates adaptive quizzes, tracks performance, and has built in chat with tooling to interact with your notes — available as a web interface.

![Knowledge Pipeline](./docs/flowcharts/Knowledge-Pipeline.png)
![Quiz Pipeline](./docs/flowcharts/Quiz-Pipeline.png)

![ForgeAI Demo](./ForgeAI.gif)

---

## Requirements

- Go 1.21+
- Node.js 20+ and npm (for the web UI)
- [`studyforge`](https://github.com/Jadog1/study-forge) installed and on `$PATH`
- An API key for OpenAI, Anthropic Claude, VoyageAI, or a locally running Ollama instance

---

## Installation

```bash
go install ./cmd/sfa
```

---

## Web UI

The web interface provides the full Study Forge AI experience in the browser
with interactive charts, rich text editing, and a modern dashboard.

### Quick start (production)

```bash
# 1. Build the frontend
cd web && npm install && npm run build && cd ..

# 2. Start the server (serves both API and frontend)
sfa web
```

This opens `http://localhost:8080` in your browser automatically.

### Development mode

Run the Go API server and Vite dev server separately for hot-reload:

```bash
# Terminal 1 — API server
go install ./cmd/sfa && sfa web --dev

# Terminal 2 — Vite dev server with hot-reload
cd web && npm run dev
```

The Vite dev server runs on `http://localhost:5173` and proxies `/api` requests
to the Go server on port 8080.

### Web command flags

| Flag | Default | Description |
| --- | --- | --- |
| `--port, -p` | `8080` | Port for the HTTP server |
| `--dev` | `false` | Development mode (API only, frontend via Vite) |
| `--no-browser` | `false` | Don't auto-open browser on startup |

### Web UI pages

| Page | Description |
| --- | --- |
| **Chat** | Streaming AI chat with markdown rendering and tool-call indicators |
| **Knowledge** | Browse ingested sections and components with search, filtering, and quiz performance metrics |
| **Quiz Dashboard** | Analytics with charts, quiz history, coverage tracking, and quiz generation with live progress |
| **Classes** | Full class management — syllabus, context profiles, note roster, coverage scopes |
| **Usage** | Token and cost analytics with time filters, per-model breakdown, trend charts, and CSV export |
| **Settings** | Provider/model configuration, per-role overrides, embeddings, SFQ settings, and model pricing |

---

## Configuration — `~/.study-forge-ai/config.yaml`

```yaml
provider: openai          # openai | claude | local

embeddings:
  provider: openai        # openai | voyage | local
  model: text-embedding-3-small

openai:
  api_key: sk-...
  model: gpt-4o

claude:
  api_key: sk-ant-...
  model: claude-3-5-sonnet-20241022

voyage:
  api_key: pa-...
  model: voyage-3-large

local:
  endpoint: http://localhost:11434  # Ollama base URL or OpenAI-compatible base URL
  embeddings_endpoint: http://localhost:8000/v1/embeddings
  model: llama3

sfq:
  command: sfq

# Appended verbatim to every AI prompt.
custom_prompt_context: |
  Focus on conceptual understanding over memorisation.
  Include real-world analogies where possible.
```

> **Security**: `~/.study-forge-ai/config.yaml` lives outside the repo so API keys stay out of version control by default. Never copy or symlink this file into the repo.

### Recommended: use environment variables

API keys are read from environment variables:

| Environment variable | Provider |
| --- | --- |
| `OPENAI_API_KEY_SFA` | OpenAI |
| `ANTHROPIC_API_KEY_SFA` | Anthropic Claude |
| `VOYAGE_API_KEY_SFA` | VoyageAI |

---

## AI Provider Plugins

Each provider lives in `plugins/<name>/` and implements the `AIProvider` interface:

```go
type AIProvider interface {
    Generate(prompt string) (string, error)
    Name() string
}
```

| Plugin | Location | Backend |
| --- | --- | --- |
| `openai` | `plugins/openai/` | OpenAI Chat Completions |
| `claude` | `plugins/claude/` | Anthropic Messages API |
| `local` | `plugins/local/` | Ollama `/api/generate` |

---

## Quiz Format (studyforge input)

All generated quizzes conform to this structure so `studyforge` can render them:

```yaml
title: Example Quiz
class: linear-algebra
tags:
  - vectors
sections:
  - type: question
    id: q-001
    question: What is a vector?
    hint: Think magnitude and direction.
    answer: A quantity with both magnitude and direction.
    reasoning: Vectors represent directional quantities in physics and math.
    tags:
      - vectors
      - fundamentals
```

---

## Example Workflow

```text
Ingest notes → extract metadata → save to index
         ↓
Generate quiz (using summaries + weak-area awareness)
         ↓
Study / render with studyforge
         ↓
Complete quiz (record correct / incorrect)
         ↓
Adapt: generate targeted follow-up questions on weak tags
         ↓
Repeat
```
