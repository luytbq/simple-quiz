package db

import (
	"fmt"
	"testing"
)

var migrateTestCounter int64

// TestBackfillDefaultChapters simulates a legacy database where questions have
// no chapter_id, then re-runs Migrate and asserts a "Mặc định" chapter is
// created and assigned.
func TestBackfillDefaultChapters(t *testing.T) {
	migrateTestCounter++
	dsn := fmt.Sprintf("file:migratetest%d?mode=memory&cache=shared", migrateTestCounter)
	d, err := Open(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Simulate legacy rows: a subject with questions that have no chapter_id.
	res, err := d.Exec("INSERT INTO subjects (name, share_code) VALUES ('Legacy', 'code1234')")
	if err != nil {
		t.Fatalf("insert subject: %v", err)
	}
	subjectID, _ := res.LastInsertId()
	for i := 0; i < 3; i++ {
		if _, err := d.Exec(
			"INSERT INTO questions (subject_id, content) VALUES (?, ?)",
			subjectID, fmt.Sprintf("Q%d", i),
		); err != nil {
			t.Fatalf("insert question: %v", err)
		}
	}

	var nullCount int
	d.QueryRow("SELECT COUNT(*) FROM questions WHERE chapter_id IS NULL").Scan(&nullCount)
	if nullCount != 3 {
		t.Fatalf("setup: expected 3 chapter-less questions, got %d", nullCount)
	}

	// Re-run migration: backfill should create and assign a default chapter.
	if err := Migrate(d); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var chapterCount int
	d.QueryRow("SELECT COUNT(*) FROM chapters WHERE subject_id = ?", subjectID).Scan(&chapterCount)
	if chapterCount != 1 {
		t.Errorf("expected 1 default chapter, got %d", chapterCount)
	}

	var name string
	var importance int
	d.QueryRow("SELECT name, importance FROM chapters WHERE subject_id = ?", subjectID).Scan(&name, &importance)
	if name != DefaultChapterName {
		t.Errorf("default chapter name = %q, want %q", name, DefaultChapterName)
	}
	if importance != DefaultImportance {
		t.Errorf("default chapter importance = %d, want %d", importance, DefaultImportance)
	}

	d.QueryRow("SELECT COUNT(*) FROM questions WHERE chapter_id IS NULL").Scan(&nullCount)
	if nullCount != 0 {
		t.Errorf("expected all questions assigned a chapter, %d still null", nullCount)
	}

	// Idempotent: running again creates no extra chapters.
	if err := Migrate(d); err != nil {
		t.Fatalf("third migrate: %v", err)
	}
	d.QueryRow("SELECT COUNT(*) FROM chapters WHERE subject_id = ?", subjectID).Scan(&chapterCount)
	if chapterCount != 1 {
		t.Errorf("backfill not idempotent: %d chapters after re-run", chapterCount)
	}
}
