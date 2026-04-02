package service

import (
	"database/sql"
	"fmt"
	"testing"

	"quiz/internal/db"
)

var testDBCounter int64

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// Use a unique shared in-memory DB per test to avoid connection pool issues.
	// Each in-memory DB needs a unique name so tests don't share state.
	testDBCounter++
	dsn := fmt.Sprintf("file:testdb%d?mode=memory&cache=shared", testDBCounter)
	d, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func sampleImportData() db.ImportData {
	return db.ImportData{
		Subject: "Go Basics",
		Questions: []db.ImportQuestion{
			{
				Content:     "What is Go?",
				Explanation: "Go is a programming language.",
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "A programming language", IsCorrect: true},
					{Label: "B", Content: "A database", IsCorrect: false},
					{Label: "C", Content: "An OS", IsCorrect: false},
				},
			},
			{
				Content: "Who created Go?",
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "Google", IsCorrect: true},
					{Label: "B", Content: "Microsoft", IsCorrect: false},
				},
			},
		},
	}
}

func TestImportQuestions(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	data := sampleImportData()
	sub, count, err := qs.ImportQuestions(data)
	if err != nil {
		t.Fatalf("ImportQuestions: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 questions imported, got %d", count)
	}
	if sub.Name != "Go Basics" {
		t.Errorf("expected subject name 'Go Basics', got %q", sub.Name)
	}
	if sub.QuestionCount != 2 {
		t.Errorf("expected QuestionCount=2, got %d", sub.QuestionCount)
	}
	if sub.ShareCode == "" {
		t.Error("expected share code to be generated on import")
	}
}

func TestImportQuestions_ExistingSubject(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	data := sampleImportData()
	sub1, count1, err := qs.ImportQuestions(data)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}

	// Import again with same subject name but different questions
	data2 := db.ImportData{
		Subject: "Go Basics",
		Questions: []db.ImportQuestion{
			{
				Content: "What is a goroutine?",
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "A lightweight thread", IsCorrect: true},
					{Label: "B", Content: "A variable", IsCorrect: false},
				},
			},
		},
	}
	sub2, count2, err := qs.ImportQuestions(data2)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if count2 != 1 {
		t.Errorf("expected 1 question in second import, got %d", count2)
	}
	if sub2.ID != sub1.ID {
		t.Errorf("expected same subject ID %d, got %d", sub1.ID, sub2.ID)
	}
	// Total questions should now be 3
	if sub2.QuestionCount != count1+count2 {
		t.Errorf("expected total %d questions, got %d", count1+count2, sub2.QuestionCount)
	}
}

func TestListSubjects(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	// Empty initially
	subjects, err := qs.ListSubjects()
	if err != nil {
		t.Fatalf("ListSubjects: %v", err)
	}
	if len(subjects) != 0 {
		t.Errorf("expected 0 subjects, got %d", len(subjects))
	}

	// Import some
	qs.ImportQuestions(sampleImportData())
	subjects, err = qs.ListSubjects()
	if err != nil {
		t.Fatalf("ListSubjects: %v", err)
	}
	if len(subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(subjects))
	}
	if subjects[0].Name != "Go Basics" {
		t.Errorf("expected 'Go Basics', got %q", subjects[0].Name)
	}
	if subjects[0].QuestionCount != 2 {
		t.Errorf("expected QuestionCount=2, got %d", subjects[0].QuestionCount)
	}
}

func TestGetSubject(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, _, _ := qs.ImportQuestions(sampleImportData())

	got, err := qs.GetSubject(sub.ID)
	if err != nil {
		t.Fatalf("GetSubject: %v", err)
	}
	if got.Name != "Go Basics" {
		t.Errorf("expected 'Go Basics', got %q", got.Name)
	}
	if got.QuestionCount != 2 {
		t.Errorf("expected QuestionCount=2, got %d", got.QuestionCount)
	}
}

func TestGetSubject_NotFound(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	_, err := qs.GetSubject(999)
	if err == nil {
		t.Fatal("expected error for non-existent subject")
	}
}

func TestGetRandomQuestion(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, _, _ := qs.ImportQuestions(sampleImportData())

	q, err := qs.GetRandomQuestion(sub.ID, nil)
	if err != nil {
		t.Fatalf("GetRandomQuestion: %v", err)
	}
	if q.SubjectID != sub.ID {
		t.Errorf("expected SubjectID=%d, got %d", sub.ID, q.SubjectID)
	}
	if len(q.Answers) == 0 {
		t.Error("expected answers to be loaded")
	}
}

func TestGetRandomQuestion_ExcludeIDs(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, _, _ := qs.ImportQuestions(sampleImportData())

	// Get first question
	q1, err := qs.GetRandomQuestion(sub.ID, nil)
	if err != nil {
		t.Fatalf("GetRandomQuestion: %v", err)
	}

	// Get second question excluding first
	q2, err := qs.GetRandomQuestion(sub.ID, []int64{q1.ID})
	if err != nil {
		t.Fatalf("GetRandomQuestion with exclude: %v", err)
	}
	if q2.ID == q1.ID {
		t.Error("expected different question when excluding first")
	}

	// Exclude both — should fail
	_, err = qs.GetRandomQuestion(sub.ID, []int64{q1.ID, q2.ID})
	if err == nil {
		t.Error("expected error when all questions excluded")
	}
}

func TestCountQuestions(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, _, _ := qs.ImportQuestions(sampleImportData())
	count, err := qs.CountQuestions(sub.ID)
	if err != nil {
		t.Fatalf("CountQuestions: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestDeleteSubject(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, _, _ := qs.ImportQuestions(sampleImportData())

	err := qs.DeleteSubject(sub.ID)
	if err != nil {
		t.Fatalf("DeleteSubject: %v", err)
	}

	_, err = qs.GetSubject(sub.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}

	count, _ := qs.CountQuestions(sub.ID)
	if count != 0 {
		t.Errorf("expected 0 questions after deletion, got %d", count)
	}
}

func TestExportSubject_Roundtrip(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	original := sampleImportData()
	sub, _, err := qs.ImportQuestions(original)
	if err != nil {
		t.Fatalf("ImportQuestions: %v", err)
	}

	exported, err := qs.ExportSubject(sub.ID)
	if err != nil {
		t.Fatalf("ExportSubject: %v", err)
	}

	if exported.Subject != original.Subject {
		t.Errorf("subject mismatch: got %q, want %q", exported.Subject, original.Subject)
	}
	if len(exported.Questions) != len(original.Questions) {
		t.Fatalf("question count mismatch: got %d, want %d", len(exported.Questions), len(original.Questions))
	}

	for i, eq := range exported.Questions {
		oq := original.Questions[i]
		if eq.Content != oq.Content {
			t.Errorf("Q%d content mismatch: got %q, want %q", i+1, eq.Content, oq.Content)
		}
		if len(eq.Answers) != len(oq.Answers) {
			t.Errorf("Q%d answer count mismatch: got %d, want %d", i+1, len(eq.Answers), len(oq.Answers))
			continue
		}
		for j, ea := range eq.Answers {
			oa := oq.Answers[j]
			if ea.Label != oa.Label {
				t.Errorf("Q%d A%d label mismatch: got %q, want %q", i+1, j+1, ea.Label, oa.Label)
			}
			if ea.Content != oa.Content {
				t.Errorf("Q%d A%d content mismatch: got %q, want %q", i+1, j+1, ea.Content, oa.Content)
			}
			if ea.IsCorrect != oa.IsCorrect {
				t.Errorf("Q%d A%d is_correct mismatch: got %v, want %v", i+1, j+1, ea.IsCorrect, oa.IsCorrect)
			}
		}
	}
}

func TestEnsureShareCode(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, _, _ := qs.ImportQuestions(sampleImportData())

	// ImportQuestions already generates a share code
	code1, err := qs.EnsureShareCode(sub.ID)
	if err != nil {
		t.Fatalf("EnsureShareCode: %v", err)
	}
	if code1 == "" {
		t.Fatal("expected non-empty share code")
	}
	if len(code1) != 8 {
		t.Errorf("expected 8-char code, got %d chars: %q", len(code1), code1)
	}

	// Idempotent — calling again should return same code
	code2, err := qs.EnsureShareCode(sub.ID)
	if err != nil {
		t.Fatalf("EnsureShareCode second call: %v", err)
	}
	if code2 != code1 {
		t.Errorf("expected same code %q, got %q", code1, code2)
	}
}

func TestGetSubjectByShareCode(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, _, _ := qs.ImportQuestions(sampleImportData())
	code, _ := qs.EnsureShareCode(sub.ID)

	got, err := qs.GetSubjectByShareCode(code)
	if err != nil {
		t.Fatalf("GetSubjectByShareCode: %v", err)
	}
	if got.ID != sub.ID {
		t.Errorf("expected ID=%d, got %d", sub.ID, got.ID)
	}
	if got.Name != sub.Name {
		t.Errorf("expected name=%q, got %q", sub.Name, got.Name)
	}
}

func TestGetSubjectByShareCode_NotFound(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	_, err := qs.GetSubjectByShareCode("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent share code")
	}
}

func TestImportQuestions_MultiAnswerAutoDetection(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	data := db.ImportData{
		Subject: "Multi",
		Questions: []db.ImportQuestion{
			{
				Content: "Select all that apply:",
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "Correct 1", IsCorrect: true},
					{Label: "B", Content: "Correct 2", IsCorrect: true},
					{Label: "C", Content: "Wrong", IsCorrect: false},
				},
				// MultiAnswer is nil — should be auto-detected as true
			},
			{
				Content: "Single answer:",
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "Correct", IsCorrect: true},
					{Label: "B", Content: "Wrong", IsCorrect: false},
				},
				// MultiAnswer is nil — should be auto-detected as false
			},
		},
	}

	sub, _, err := qs.ImportQuestions(data)
	if err != nil {
		t.Fatalf("ImportQuestions: %v", err)
	}

	// Verify multi_answer flag via GetRandomQuestion repeatedly or direct query
	var multiAnswer bool
	// Check first question (multi-answer)
	rows, err := d.Query("SELECT multi_answer FROM questions WHERE subject_id = ? ORDER BY order_number", sub.ID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	i := 0
	expected := []bool{true, false}
	for rows.Next() {
		if err := rows.Scan(&multiAnswer); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if multiAnswer != expected[i] {
			t.Errorf("question %d: expected multi_answer=%v, got %v", i+1, expected[i], multiAnswer)
		}
		i++
	}
	if i != 2 {
		t.Errorf("expected 2 questions, got %d", i)
	}
}

func TestImportQuestions_MultiAnswerExplicit(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	f := false
	data := db.ImportData{
		Subject: "Explicit",
		Questions: []db.ImportQuestion{
			{
				Content:     "Explicitly not multi:",
				MultiAnswer: &f,
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "Correct 1", IsCorrect: true},
					{Label: "B", Content: "Correct 2", IsCorrect: true},
					{Label: "C", Content: "Wrong", IsCorrect: false},
				},
			},
		},
	}

	sub, _, err := qs.ImportQuestions(data)
	if err != nil {
		t.Fatalf("ImportQuestions: %v", err)
	}

	var multiAnswer bool
	err = d.QueryRow("SELECT multi_answer FROM questions WHERE subject_id = ?", sub.ID).Scan(&multiAnswer)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if multiAnswer != false {
		t.Error("expected multi_answer=false when explicitly set, got true")
	}
}
