# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o quiz .

# Run web server (default :8080, configurable via PORT env)
./quiz
# or
DB_PATH=./quiz.db PORT=8080 ./quiz

# Import questions from JSON file
./quiz import <file.json>

# Docker
docker compose up --build
```

## Architecture

Single-binary Go web app (Go 1.22+ ServeMux, html/template, modernc.org/sqlite). No JS frameworks, no auth. Templates and static files are embedded via `//go:embed` in `main.go`.

**Entry point** (`main.go`): Dispatches between CLI import (`os.Args[1] == "import"`) and web server (default).

**Layers**:
- `internal/db/` — SQLite connection, migrations (auto-run on startup), and struct definitions
- `internal/service/` — Business logic. `QuestionService` (CRUD, import, random selection) and `AttemptService` (attempt tracking, scoring)
- `internal/handler/` — HTTP handlers. `Handler` struct holds services + parsed templates. Each page template is parsed separately with `layout.html` to avoid `{{define "content"}}` conflicts
- `cmd/import.go` — CLI import logic, called from `main.go`

**Template pattern**: Each page defines `{{define "content"}}...{{end}}`. Handler parses each page individually with `layout.html` into `map[string]*template.Template`, then renders via `ExecuteTemplate(w, "layout", data)`.

**Database**: SQLite with WAL mode. 5 tables: `subjects`, `questions`, `answers`, `exam_attempts`, `attempt_answers`. Schema lives in `internal/db/db.go` as a const string, auto-migrated via `CREATE TABLE IF NOT EXISTS`.

**JSON import format**:
```json
{
  "subject": "Subject Name",
  "questions": [
    {
      "content": "Question?",
      "answers": [
        {"label": "A", "content": "...", "is_correct": false},
        {"label": "B", "content": "...", "is_correct": true}
      ]
    }
  ]
}
```

**Key behaviors**:
- Flashcard mode avoids repeat questions by tracking answered IDs in `attempt_answers` and excluding them via `NOT IN`
- Answer order is shuffled at read time (not stored), using `math/rand/v2`
- Subject import is upsert: if subject name exists, questions are appended
