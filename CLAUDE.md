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

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/service/ -v
go test ./internal/handler/ -v
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

**Database**: SQLite with WAL mode. 7 tables: `subjects`, `chapters`, `questions`, `answers`, `exam_attempts`, `attempt_answers`, `attempt_chapters`. Schema lives in `internal/db/db.go` as a const string, auto-migrated via `CREATE TABLE IF NOT EXISTS` + idempotent `ALTER TABLE ... ADD COLUMN`. `Migrate()` also backfills a "Mặc định" chapter for any pre-chapters questions (`backfillDefaultChapters`).

**JSON import format**:
```json
{
  "subject": "Subject Name",
  "chapters": [
    {"id": 1, "name": "Chapter 1", "importance": 4}
  ],
  "questions": [
    {
      "content": "Question?",
      "chapter_id": 1,
      "answers": [
        {"label": "A", "content": "...", "is_correct": false},
        {"label": "B", "content": "...", "is_correct": true}
      ]
    }
  ]
}
```
`chapters` and `chapter_id` are optional; when absent, questions go into one auto-created "Mặc định" chapter (importance 5). Chapter fields are never rejected — `normalizeChapters` (`internal/service/chapter.go`) fills a missing name from the id, clamps importance to 1-10, and auto-creates undeclared `chapter_id`s.

## Rules

- When adding or updating features, always check and update the guide page (`templates/guide.html`) and README files if the change affects user-facing behavior or input specs.
- When adding or modifying service/handler logic, add or update unit tests. Tests use in-memory SQLite (`file:testN?mode=memory&cache=shared`). Run `go test ./...` before committing.

**Key behaviors**:
- Flashcard mode avoids repeat questions by tracking answered IDs in `attempt_answers` and excluding them via `NOT IN`
- Answer order is shuffled at read time (not stored), using `math/rand/v2`
- Subject import is upsert: if subject name exists, questions are appended
- Chapters: each question belongs to a chapter with `importance` 1-10 (default 5). Setup pages (`exam_setup`, `practice_setup`, `share`) show a chapter chooser only when a subject has >1 chapter. The selected chapter ids are persisted per attempt in `attempt_chapters` so `ExamTake`/`PracticeQuestion` can re-apply the filter.
- Exam question allocation across chapters lives in `allocateQuestions` (`internal/service/chapter.go`): distributes the requested count by importance, guaranteeing (in priority order) exact total → ≥1 per selected chapter → proportional split, with each chapter capped at its available questions.
