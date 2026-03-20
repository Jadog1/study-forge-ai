# study-agent

An AI-powered study assistant written in Go with a baked-in Bubble Tea workflow.
It ingests notes, extracts structured knowledge, generates quizzes, tracks
performance, streams chat responses in-app, and integrates sfq search.

---

## Requirements

- Go 1.21+
- [`studyforge`](https://github.com/studyforge/studyforge) installed and on `$PATH`
- An API key for OpenAI, Anthropic Claude, VoyageAI, or a locally running Ollama instance

---

## Installation

```bash
go install ./cmd/sfa
```

Launch the interactive workflow:

```bash
sfa
```

Example kicking off local LLM:

```bash
python ./example_local_llm/llm.py
```

---

## Interactive Workflow

The default experience is a multi-pane Bubble Tea app:

- Chat: stream model responses in a chat window
- Classes: create/select classes and attach context files
- Settings: manage provider keys/models and sfq command
- SFQ Search: run sfq plugin searches without leaving the app

Controls:

- tab / shift+tab: switch panes
- q: quit
- esc: leave edit mode

Pane-specific controls:

- Classes pane: n (new class), a (add context file), up/down (select class)
- Settings pane: up/down (select setting), e (edit), s (save config)
- SFQ pane: enter to run search

## CLI Quick Start

```bash
# 1. Optional: pre-create app data
sfa init

# App data is also created automatically the first time you run any command.
# Edit ~/.study-forge-ai/config.yaml — add your API key and choose a provider

# 2. Create a class
sfa class create linear-algebra

# 3. Ingest notes
sfa ingest ./notes/math --class linear-algebra

# 4. Generate a quiz
sfa generate linear-algebra

# 5. Study the quiz
sfa study ~/.study-forge-ai/quizzes/linear-algebra/quiz-<id>.yaml

# 6. Render to HTML via studyforge
studyforge build ~/.study-forge-ai/quizzes/linear-algebra/quiz-<id>.yaml

# 7. Record your results
sfa complete ~/.study-forge-ai/quizzes/linear-algebra/quiz-<id>.yaml

# 8. Generate adaptive follow-up quiz
sfa adapt linear-algebra
```

---

## CLI Commands

| Command | Description |
| --- | --- |
| `sfa init` | Initialise `~/.study-forge-ai/` app data |
| `sfa ingest <path> [--class <name>]` | Ingest and process notes from a folder |
| `sfa generate <class> [--tags ...]` | Generate a quiz from ingested notes |
| `sfa study <quiz-path>` | Print quiz questions to the terminal |
| `sfa complete <quiz-path>` | Record quiz results interactively |
| `sfa adapt <class>` | Generate adaptive quiz from performance data |
| `sfa search [--tags ...] [--class ...]` | Search ingested notes |
| `sfa class create <name>` | Create a new class |
| `sfa class list` | List all classes |

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
  command: studyforge sfq

# Appended verbatim to every AI prompt.
custom_prompt_context: |
  Focus on conceptual understanding over memorisation.
  Include real-world analogies where possible.
```

> **Security**: `~/.study-forge-ai/config.yaml` lives outside the repo so API keys stay out of version control by default. Never copy or symlink this file into the repo.

### Recommended: use environment variables

For stronger security (CI, shared machines, dotfile repos), set keys as environment variables instead of storing them in the config file. Environment variables always take precedence over the config file at runtime:

| Environment variable | Provider |
| --- | --- |
| `OPENAI_API_KEY_SFA` | OpenAI |
| `ANTHROPIC_API_KEY_SFA` | Anthropic Claude |
| `VOYAGE_API_KEY_SFA` | VoyageAI |

API keys are **exclusively** sourced from these environment variables — they are never read from `config.yaml` and are always stripped before any write to disk. The `api_key` fields in `config.yaml` will always be empty. A good place to set the vars is your shell profile (`~/.bashrc`, `~/.zshrc`):

```bash
export OPENAI_API_KEY_SFA="sk-..."
export ANTHROPIC_API_KEY_SFA="sk-ant-..."
export VOYAGE_API_KEY_SFA="pa-..."
```

---

## Workspace Layout

```text
~/.study-forge-ai/
  config.yaml              ← provider credentials
  notes/
    raw/                   ← copy raw notes here (optional)
    processed/             ← AI-extracted metadata (YAML + index.json)
  classes/
    <class>/
      syllabus.yaml        ← weekly topics
      rules.yaml           ← exam style expectations
      context.yaml         ← file paths to inject as class context in chat/AI
  quizzes/
    <class>/
      quiz-<ts>.yaml       ← generated quiz (studyforge input)
      quiz-<ts>-results.json
  plans/
  cache/
```

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

---

## Extending Prompts

Edit `internal/prompts/prompts.go` to adjust any of the four built-in templates:

| Function | Purpose |
| --- | --- |
| `SummarizeNote` | Extracts summary, tags, and concepts from raw notes |
| `GenerateQuestions` | Produces a full quiz YAML document |
| `AdaptQuestions` | Generates weak-area targeted follow-ups |
| `VariationQuestion` | Reframes an existing question with a new angle |

Or use `custom_prompt_context` in `~/.study-forge-ai/config.yaml` to append instructions without
touching the code.
