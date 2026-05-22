package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("exec %s: %w", p, err)
		}
	}

	return db, nil
}

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	// Add columns that may not exist in older databases
	for _, col := range []string{
		"ALTER TABLE questions ADD COLUMN explanation TEXT DEFAULT ''",
		"ALTER TABLE questions ADD COLUMN multi_answer BOOLEAN NOT NULL DEFAULT 0",
		"ALTER TABLE subjects ADD COLUMN share_code TEXT DEFAULT ''",
		// SQLite allows adding a column with a foreign key as long as the default is NULL.
		"ALTER TABLE questions ADD COLUMN chapter_id INTEGER REFERENCES chapters(id)",
	} {
		db.Exec(col) // ignore "duplicate column" errors
	}
	return backfillDefaultChapters(db)
}

// backfillDefaultChapters ensures every question belongs to a chapter. For any
// subject that still has chapter-less questions (older databases), it creates a
// single "Mặc định" chapter and assigns those questions to it. Idempotent: once
// every question has a chapter_id this is a no-op.
func backfillDefaultChapters(db *sql.DB) error {
	rows, err := db.Query("SELECT DISTINCT subject_id FROM questions WHERE chapter_id IS NULL")
	if err != nil {
		return err
	}
	var subjectIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		subjectIDs = append(subjectIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, subjectID := range subjectIDs {
		res, err := db.Exec(
			"INSERT INTO chapters (subject_id, name, importance, order_number) VALUES (?, ?, ?, ?)",
			subjectID, DefaultChapterName, DefaultImportance, 0,
		)
		if err != nil {
			return err
		}
		chapterID, _ := res.LastInsertId()
		if _, err := db.Exec(
			"UPDATE questions SET chapter_id = ? WHERE subject_id = ? AND chapter_id IS NULL",
			chapterID, subjectID,
		); err != nil {
			return err
		}
	}
	return nil
}

// DefaultChapterName is the chapter that holds questions without an explicit
// chapter (legacy data and imports that omit chapter info).
const DefaultChapterName = "Mặc định"

// DefaultImportance is used when a chapter's importance is missing or zero.
const DefaultImportance = 5

const schema = `
CREATE TABLE IF NOT EXISTS subjects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    share_code TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chapters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_id INTEGER NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    importance INTEGER NOT NULL DEFAULT 5,
    order_number INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS questions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_id INTEGER NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    chapter_id INTEGER REFERENCES chapters(id),
    content TEXT NOT NULL,
    explanation TEXT DEFAULT '',
    multi_answer BOOLEAN NOT NULL DEFAULT 0,
    order_number INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS answers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    content TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS exam_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_id INTEGER NOT NULL REFERENCES subjects(id),
    mode TEXT NOT NULL CHECK(mode IN ('flashcard', 'exam')),
    score REAL DEFAULT 0,
    total_questions INTEGER DEFAULT 0,
    correct_count INTEGER DEFAULT 0,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME
);

CREATE TABLE IF NOT EXISTS attempt_answers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    attempt_id INTEGER NOT NULL REFERENCES exam_attempts(id) ON DELETE CASCADE,
    question_id INTEGER NOT NULL REFERENCES questions(id),
    selected_answer_id INTEGER REFERENCES answers(id)
);

CREATE TABLE IF NOT EXISTS attempt_chapters (
    attempt_id INTEGER NOT NULL REFERENCES exam_attempts(id) ON DELETE CASCADE,
    chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    PRIMARY KEY (attempt_id, chapter_id)
);
`
