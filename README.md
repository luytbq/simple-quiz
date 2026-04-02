# Quiz App

Personal quiz/practice app for self-study. Import AI-generated question sets and practice via flashcard or exam mode.

[Tiếng Việt](README.vi.md)

## Quick Start

```bash
# Build
go build -o quiz .

# Import questions
./quiz import questions.json

# Start server
./quiz
# Open http://localhost:8080
```

### Docker

```bash
docker compose up --build
# Open http://localhost:8080
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `DB_PATH` | `quiz.db` | SQLite database file path |
| `BASE_PATH` | (empty) | URL prefix for reverse proxy (e.g. `/exams`) |

## Usage

### Import Questions

**Web UI:** Go to `/manage`, paste JSON, click "Kiểm tra" to preview, then "Xác nhận Import".

**CLI:** `./quiz import questions.json`

The app auto-fixes common issues: strips markdown wrappers, trailing commas, extra text around JSON, and single-backtick code blocks.

### Practice Modes

**Flashcard** — One question at a time, instant feedback, no repeats within a session.

**Exam** — Choose question count, answer all, submit, get score + full review.

### Share

Each subject gets a share link (`/s/{code}`). Anyone with the link can practice — no login needed.

---

## Question JSON Specification

> **For AI agents:** This section is the complete specification for generating question sets. Read this section to produce valid JSON output. No other context is needed.

### Schema

```json
{
  "subject": "string (required) — topic name",
  "questions": [
    {
      "content": "string (required) — question text, supports Markdown",
      "explanation": "string (optional) — shown after answering, only when answer is non-obvious",
      "multi_answer": "boolean (optional) — auto-detected if omitted",
      "answers": [
        {
          "label": "string (required) — e.g. A, B, C, D",
          "content": "string (required) — answer text, supports inline Markdown",
          "is_correct": "boolean (required) — true for correct answer(s)"
        }
      ]
    }
  ]
}
```

### Validation Rules

1. `subject`: non-empty string
2. `questions`: non-empty array
3. Each question must have `content` (non-empty) and `answers` (>= 2 items)
4. Each answer must have `label` (unique within question), `content`, and `is_correct`
5. Each question must have **at least one** answer with `is_correct: true`
6. If multiple answers have `is_correct: true`, the question is automatically treated as multi-answer (checkboxes instead of radio buttons)

### Markdown Formatting

Content fields (`content`, `explanation`) support Markdown rendered server-side. Rules:

| Element | Syntax in JSON string | Use when |
|---------|----------------------|----------|
| Code block | `\n```java\ncode\n```\n` | Question contains source code |
| Inline code | `` `value` `` | Answer is a code value |
| Bold | `**text**` | Emphasis |
| Mermaid diagram | `\n```mermaid\ngraph TD;\nA-->B;\n```\n` | Structured diagrams: flowchart, UML, ER, sequence |
| ASCII art | `\n```\n[diagram]\n```\n` | Diagrams Mermaid can't render: stack, memory, truth table, binary tree |

**Critical:** Code blocks MUST use triple backticks (` ``` `), NOT single backticks. Newlines in JSON strings use `\n`.

### Complete Example

```json
{
  "subject": "Java OOP",
  "questions": [
    {
      "content": "What does this code print?\n\n```java\nString s = \"hello\";\nSystem.out.println(s.toUpperCase());\n```",
      "explanation": "`toUpperCase()` returns a **new String**, does not modify the original.",
      "answers": [
        {"label": "A", "content": "`HELLO`", "is_correct": true},
        {"label": "B", "content": "`hello`", "is_correct": false},
        {"label": "C", "content": "`Hello`", "is_correct": false},
        {"label": "D", "content": "Compilation error", "is_correct": false}
      ]
    },
    {
      "content": "Which diagram represents the Observer pattern?\n\n```mermaid\nclassDiagram\n  class Subject {\n    +attach(Observer)\n    +notify()\n  }\n  class Observer {\n    +update()\n  }\n  Subject o-- Observer\n```",
      "answers": [
        {"label": "A", "content": "Observer", "is_correct": true},
        {"label": "B", "content": "Strategy", "is_correct": false},
        {"label": "C", "content": "Factory", "is_correct": false},
        {"label": "D", "content": "Singleton", "is_correct": false}
      ]
    },
    {
      "content": "After `push(1)`, `push(2)`, `push(3)`, `pop()`, the stack is:\n\n```\n| 2 | ← top\n| 1 |\n+---+\n```\n\nWhat does the next `pop()` return?",
      "answers": [
        {"label": "A", "content": "`2`", "is_correct": true},
        {"label": "B", "content": "`1`", "is_correct": false},
        {"label": "C", "content": "`3`", "is_correct": false},
        {"label": "D", "content": "Stack is empty", "is_correct": false}
      ]
    },
    {
      "content": "Which are valid access modifiers in Java?",
      "multi_answer": true,
      "answers": [
        {"label": "A", "content": "`public`", "is_correct": true},
        {"label": "B", "content": "`private`", "is_correct": true},
        {"label": "C", "content": "`internal`", "is_correct": false},
        {"label": "D", "content": "`protected`", "is_correct": true}
      ]
    }
  ]
}
```

### Generating Questions with AI

Give the AI agent the following information:
1. The **topic** and **number of questions**
2. The **JSON specification** above (or a link to this README)
3. Save the output to a `.json` file and run `./quiz import file.json`, or paste into the web UI at `/manage`

The app's import flow auto-corrects common AI output issues (markdown wrappers, trailing commas, wrong backtick usage).
