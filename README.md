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

Data is persisted in a Docker volume. To import questions inside the container:

```bash
docker compose exec quiz ./quiz import /data/questions.json
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `DB_PATH` | `quiz.db` | SQLite database file path |

## Usage

### Import Questions

**CLI:**

```bash
./quiz import questions.json
```

**Web UI:**

Go to `http://localhost:8080/import`, paste JSON into the textarea, and submit.

If the subject already exists, new questions will be appended to it.

### Practice Modes

**Flashcard** — Answer one question at a time, see the result immediately, then move to the next. Questions are shuffled and not repeated within a session.

**Exam** — Choose the number of questions, answer all of them, submit, and get a score with a full review of correct/incorrect answers.

### Statistics

View per-subject accuracy, attempt history, and best/average scores at `/stats`.

## Input Specs

Questions are imported as JSON with this structure:

```json
{
  "subject": "Subject Name",
  "questions": [
    {
      "content": "Question text goes here?",
      "explanation": "Optional explanation shown after answering",
      "answers": [
        {"label": "A", "content": "First option", "is_correct": false},
        {"label": "B", "content": "Second option", "is_correct": true},
        {"label": "C", "content": "Third option", "is_correct": false},
        {"label": "D", "content": "Fourth option", "is_correct": false}
      ]
    }
  ]
}
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `subject` | string | yes | Subject/topic name. If it already exists, questions are appended |
| `questions` | array | yes | List of question objects |
| `questions[].content` | string | yes | The question text (supports Markdown: code blocks, bold, images) |
| `questions[].explanation` | string | no | Explanation shown in result review (supports Markdown). Only include when the answer is non-obvious |
| `questions[].answers` | array | yes | List of answer options (typically 4) |
| `questions[].answers[].label` | string | yes | Answer label (e.g. "A", "B", "C", "D") |
| `questions[].answers[].content` | string | yes | Answer text (supports inline Markdown: \`code\`, **bold**) |
| `questions[].answers[].is_correct` | boolean | yes | `true` for the correct answer, `false` otherwise |
| `questions[].multi_answer` | boolean | no | Set to `true` for multiple correct answers. Auto-detected if omitted (based on number of `is_correct: true` answers) |

### Rules

- Each question must have **at least one** answer with `is_correct: true`
- For multi-answer questions, multiple answers can have `is_correct: true` — the app will display checkboxes instead of radio buttons
- `multi_answer` is auto-detected if omitted: questions with 2+ correct answers are automatically treated as multi-answer
- Labels should be unique within a question (A, B, C, D)
- There is no limit on the number of answers per question, but 4 is standard
- Content fields support Markdown: \`inline code\`, \`\`\`code blocks\`\`\`, **bold**, *italic*, > blockquotes
- Diagrams via Mermaid: use \`\`\`mermaid code blocks (flowchart, UML, ER, sequence diagrams)
- ASCII art for anything Mermaid can't handle (stack frames, truth tables, trees)

## Generate Questions with AI

Copy the prompt below and paste it into any LLM (ChatGPT, Claude, Gemini, etc.). Replace the placeholders with your desired topic and quantity.

---

<pre>
Generate a set of multiple-choice questions for study/practice purposes.

Topic: [YOUR TOPIC HERE]
Number of questions: [NUMBER]
Language: [Vietnamese / English / ...]

Requirements:
- Each question has exactly 4 answer options (A, B, C, D)
- Most questions have exactly one correct answer. Some questions may have multiple correct answers — for those, mark all correct answers with "is_correct": true
- Questions should vary in difficulty (easy, medium, hard)
- Cover different aspects of the topic
- Avoid trick questions; focus on testing real understanding
- Only add "explanation" field when the answer is non-obvious, tricky, or needs clarification. Do NOT add explanation for straightforward questions.

Output ONLY valid JSON in this exact format, no explanation or markdown:

{
  "subject": "[Topic Name]",
  "questions": [
    {
      "content": "Question text?",
      "explanation": "Only if needed - explain why the answer is correct",
      "answers": [
        {"label": "A", "content": "Option A", "is_correct": false},
        {"label": "B", "content": "Option B", "is_correct": true},
        {"label": "C", "content": "Option C", "is_correct": false},
        {"label": "D", "content": "Option D", "is_correct": false}
      ]
    }
  ]
}
</pre>

---

**Example usage:**

> Generate a set of multiple-choice questions for study/practice purposes.
>
> Topic: AWS Solutions Architect Associate - S3 & Storage Services
> Number of questions: 20
> Language: English

After the LLM responds with JSON, either:
1. Save it to a file and run `./quiz import file.json`
2. Paste it directly into the web import form at `/import`
