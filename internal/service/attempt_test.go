package service

import (
	"testing"

	"quiz/internal/db"
)

func importTestSubject(t *testing.T, qs *QuestionService) *db.Subject {
	t.Helper()
	sub, _, err := qs.ImportQuestions(sampleImportData())
	if err != nil {
		t.Fatalf("ImportQuestions: %v", err)
	}
	return sub
}

func TestCreateAttempt(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)

	attempt, err := as.CreateAttempt(sub.ID, "exam", 2)
	if err != nil {
		t.Fatalf("CreateAttempt: %v", err)
	}
	if attempt.ID == 0 {
		t.Error("expected non-zero attempt ID")
	}
	if attempt.SubjectID != sub.ID {
		t.Errorf("expected SubjectID=%d, got %d", sub.ID, attempt.SubjectID)
	}
	if attempt.Mode != "exam" {
		t.Errorf("expected mode=exam, got %q", attempt.Mode)
	}
	if attempt.TotalQuestions != 2 {
		t.Errorf("expected TotalQuestions=2, got %d", attempt.TotalQuestions)
	}
	if attempt.FinishedAt != nil {
		t.Error("expected FinishedAt to be nil for new attempt")
	}
}

func TestCreateAttempt_FlashcardMode(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)

	attempt, err := as.CreateAttempt(sub.ID, "flashcard", 0)
	if err != nil {
		t.Fatalf("CreateAttempt: %v", err)
	}
	if attempt.Mode != "flashcard" {
		t.Errorf("expected mode=flashcard, got %q", attempt.Mode)
	}
}

func TestRecordAnswer(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 2)

	q, _ := qs.GetRandomQuestion(sub.ID, nil)
	answerID := q.Answers[0].ID

	err := as.RecordAnswer(attempt.ID, q.ID, &answerID)
	if err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}

	// Verify it was recorded
	ids, err := as.GetAnsweredQuestionIDs(attempt.ID)
	if err != nil {
		t.Fatalf("GetAnsweredQuestionIDs: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 answered question, got %d", len(ids))
	}
	if ids[0] != q.ID {
		t.Errorf("expected question ID %d, got %d", q.ID, ids[0])
	}
}

func TestRecordAnswer_Nil(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 2)
	q, _ := qs.GetRandomQuestion(sub.ID, nil)

	// Record a nil answer (skipped question)
	err := as.RecordAnswer(attempt.ID, q.ID, nil)
	if err != nil {
		t.Fatalf("RecordAnswer with nil: %v", err)
	}
}

func TestRecordAnswers(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 2)
	q, _ := qs.GetRandomQuestion(sub.ID, nil)

	answerIDs := []int64{q.Answers[0].ID, q.Answers[1].ID}
	err := as.RecordAnswers(attempt.ID, q.ID, answerIDs)
	if err != nil {
		t.Fatalf("RecordAnswers: %v", err)
	}

	// Verify multiple answers recorded for same question
	var count int
	d.QueryRow("SELECT COUNT(*) FROM attempt_answers WHERE attempt_id = ? AND question_id = ?",
		attempt.ID, q.ID).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 answer records, got %d", count)
	}
}

func TestRecordAnswers_Empty(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 2)
	q, _ := qs.GetRandomQuestion(sub.ID, nil)

	// Empty slice should record a nil answer
	err := as.RecordAnswers(attempt.ID, q.ID, []int64{})
	if err != nil {
		t.Fatalf("RecordAnswers empty: %v", err)
	}

	var count int
	d.QueryRow("SELECT COUNT(*) FROM attempt_answers WHERE attempt_id = ? AND question_id = ? AND selected_answer_id IS NULL",
		attempt.ID, q.ID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 null answer record, got %d", count)
	}
}

func TestFinishAttempt_AllCorrect(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 2)

	// Answer both questions correctly
	q1, _ := qs.GetRandomQuestion(sub.ID, nil)
	correct1, _ := qs.GetCorrectAnswer(q1.ID)
	as.RecordAnswer(attempt.ID, q1.ID, &correct1.ID)

	q2, _ := qs.GetRandomQuestion(sub.ID, []int64{q1.ID})
	correct2, _ := qs.GetCorrectAnswer(q2.ID)
	as.RecordAnswer(attempt.ID, q2.ID, &correct2.ID)

	finished, err := as.FinishAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("FinishAttempt: %v", err)
	}
	if finished.CorrectCount != 2 {
		t.Errorf("expected CorrectCount=2, got %d", finished.CorrectCount)
	}
	if finished.Score != 100.0 {
		t.Errorf("expected Score=100.0, got %f", finished.Score)
	}
	if finished.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
	if finished.TotalQuestions != 2 {
		t.Errorf("expected TotalQuestions=2, got %d", finished.TotalQuestions)
	}
}

func TestFinishAttempt_AllWrong(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 2)

	// Answer both questions incorrectly
	q1, _ := qs.GetRandomQuestion(sub.ID, nil)
	// Find a wrong answer
	var wrongID int64
	for _, a := range q1.Answers {
		if !a.IsCorrect {
			wrongID = a.ID
			break
		}
	}
	as.RecordAnswer(attempt.ID, q1.ID, &wrongID)

	q2, _ := qs.GetRandomQuestion(sub.ID, []int64{q1.ID})
	for _, a := range q2.Answers {
		if !a.IsCorrect {
			wrongID = a.ID
			break
		}
	}
	as.RecordAnswer(attempt.ID, q2.ID, &wrongID)

	finished, err := as.FinishAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("FinishAttempt: %v", err)
	}
	if finished.CorrectCount != 0 {
		t.Errorf("expected CorrectCount=0, got %d", finished.CorrectCount)
	}
	if finished.Score != 0.0 {
		t.Errorf("expected Score=0.0, got %f", finished.Score)
	}
}

func TestFinishAttempt_MultiAnswer(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	// Import a multi-answer question
	data := db.ImportData{
		Subject: "MultiTest",
		Questions: []db.ImportQuestion{
			{
				Content: "Select all correct:",
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "Correct 1", IsCorrect: true},
					{Label: "B", Content: "Correct 2", IsCorrect: true},
					{Label: "C", Content: "Wrong", IsCorrect: false},
				},
			},
		},
	}
	sub, _, _ := qs.ImportQuestions(data)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 1)

	q, _ := qs.GetRandomQuestion(sub.ID, nil)
	correctAnswers, _ := qs.GetCorrectAnswers(q.ID)

	// Submit exactly the correct answers
	var correctIDs []int64
	for _, a := range correctAnswers {
		correctIDs = append(correctIDs, a.ID)
	}
	as.RecordAnswers(attempt.ID, q.ID, correctIDs)

	finished, err := as.FinishAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("FinishAttempt: %v", err)
	}
	if finished.CorrectCount != 1 {
		t.Errorf("expected CorrectCount=1 (exact match), got %d", finished.CorrectCount)
	}
	if finished.Score != 100.0 {
		t.Errorf("expected Score=100.0, got %f", finished.Score)
	}
}

func TestFinishAttempt_MultiAnswer_PartialSelection(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	data := db.ImportData{
		Subject: "MultiPartial",
		Questions: []db.ImportQuestion{
			{
				Content: "Select all correct:",
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "Correct 1", IsCorrect: true},
					{Label: "B", Content: "Correct 2", IsCorrect: true},
					{Label: "C", Content: "Wrong", IsCorrect: false},
				},
			},
		},
	}
	sub, _, _ := qs.ImportQuestions(data)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 1)

	q, _ := qs.GetRandomQuestion(sub.ID, nil)
	correctAnswers, _ := qs.GetCorrectAnswers(q.ID)

	// Only select one of two correct answers — should be wrong (exact match required)
	as.RecordAnswers(attempt.ID, q.ID, []int64{correctAnswers[0].ID})

	finished, err := as.FinishAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("FinishAttempt: %v", err)
	}
	if finished.CorrectCount != 0 {
		t.Errorf("expected CorrectCount=0 (partial not accepted), got %d", finished.CorrectCount)
	}
}

func TestGetAttemptDetails(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 2)

	q, _ := qs.GetRandomQuestion(sub.ID, nil)
	correct, _ := qs.GetCorrectAnswer(q.ID)
	as.RecordAnswer(attempt.ID, q.ID, &correct.ID)
	as.FinishAttempt(attempt.ID)

	details, err := as.GetAttemptDetails(attempt.ID)
	if err != nil {
		t.Fatalf("GetAttemptDetails: %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 detail row, got %d", len(details))
	}
	if details[0].QuestionContent == "" {
		t.Error("expected question content to be populated")
	}
	if len(details[0].SelectedLabels) == 0 {
		t.Error("expected selected labels to be populated")
	}
	if len(details[0].CorrectLabels) == 0 {
		t.Error("expected correct labels to be populated")
	}
	if !details[0].IsCorrect {
		t.Error("expected IsCorrect=true for correct answer")
	}
}

func TestGetAnsweredQuestionIDs_Distinct(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	// Import multi-answer data
	data := db.ImportData{
		Subject: "Distinct",
		Questions: []db.ImportQuestion{
			{
				Content: "Q1?",
				Answers: []db.ImportAnswer{
					{Label: "A", Content: "a", IsCorrect: true},
					{Label: "B", Content: "b", IsCorrect: true},
					{Label: "C", Content: "c", IsCorrect: false},
				},
			},
		},
	}
	sub, _, _ := qs.ImportQuestions(data)
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 1)

	q, _ := qs.GetRandomQuestion(sub.ID, nil)
	// Record multiple answers for same question
	as.RecordAnswers(attempt.ID, q.ID, []int64{q.Answers[0].ID, q.Answers[1].ID})

	ids, err := as.GetAnsweredQuestionIDs(attempt.ID)
	if err != nil {
		t.Fatalf("GetAnsweredQuestionIDs: %v", err)
	}
	// Should return only 1 distinct question ID even though 2 answer rows
	if len(ids) != 1 {
		t.Errorf("expected 1 distinct question ID, got %d", len(ids))
	}
}

func TestGetRecentAttempts(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)

	// No attempts yet
	attempts, err := as.GetRecentAttempts(sub.ID, 10)
	if err != nil {
		t.Fatalf("GetRecentAttempts: %v", err)
	}
	if len(attempts) != 0 {
		t.Errorf("expected 0 attempts, got %d", len(attempts))
	}

	// Create and finish an attempt
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 1)
	q, _ := qs.GetRandomQuestion(sub.ID, nil)
	correct, _ := qs.GetCorrectAnswer(q.ID)
	as.RecordAnswer(attempt.ID, q.ID, &correct.ID)
	as.FinishAttempt(attempt.ID)

	// Unfinished attempt should not appear
	as.CreateAttempt(sub.ID, "exam", 1)

	attempts, err = as.GetRecentAttempts(sub.ID, 10)
	if err != nil {
		t.Fatalf("GetRecentAttempts: %v", err)
	}
	if len(attempts) != 1 {
		t.Errorf("expected 1 finished attempt, got %d", len(attempts))
	}
}

func TestGetSubjectStats(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	as := &AttemptService{DB: d}

	sub := importTestSubject(t, qs)

	// No stats initially
	stats, err := as.GetSubjectStats(sub.ID)
	if err != nil {
		t.Fatalf("GetSubjectStats: %v", err)
	}
	if stats.TotalAttempts != 0 {
		t.Errorf("expected 0 attempts, got %d", stats.TotalAttempts)
	}
	if stats.AvgScore != 0 {
		t.Errorf("expected AvgScore=0, got %f", stats.AvgScore)
	}

	// Create a perfect attempt
	attempt, _ := as.CreateAttempt(sub.ID, "exam", 2)
	q1, _ := qs.GetRandomQuestion(sub.ID, nil)
	c1, _ := qs.GetCorrectAnswer(q1.ID)
	as.RecordAnswer(attempt.ID, q1.ID, &c1.ID)
	q2, _ := qs.GetRandomQuestion(sub.ID, []int64{q1.ID})
	c2, _ := qs.GetCorrectAnswer(q2.ID)
	as.RecordAnswer(attempt.ID, q2.ID, &c2.ID)
	as.FinishAttempt(attempt.ID)

	stats, err = as.GetSubjectStats(sub.ID)
	if err != nil {
		t.Fatalf("GetSubjectStats: %v", err)
	}
	if stats.TotalAttempts != 1 {
		t.Errorf("expected 1 attempt, got %d", stats.TotalAttempts)
	}
	if stats.BestScore != 100.0 {
		t.Errorf("expected BestScore=100.0, got %f", stats.BestScore)
	}
	if stats.AvgScore != 100.0 {
		t.Errorf("expected AvgScore=100.0, got %f", stats.AvgScore)
	}
	if len(stats.RecentAttempts) != 1 {
		t.Errorf("expected 1 recent attempt, got %d", len(stats.RecentAttempts))
	}
}
