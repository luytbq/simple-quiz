package service

import (
	"database/sql"
	"time"

	"quiz/internal/db"
)

type AttemptService struct {
	DB *sql.DB
}

func (s *AttemptService) CreateAttempt(subjectID int64, mode string, totalQuestions int) (*db.ExamAttempt, error) {
	res, err := s.DB.Exec(
		"INSERT INTO exam_attempts (subject_id, mode, total_questions) VALUES (?, ?, ?)",
		subjectID, mode, totalQuestions,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAttempt(id)
}

func (s *AttemptService) GetAttempt(id int64) (*db.ExamAttempt, error) {
	var a db.ExamAttempt
	err := s.DB.QueryRow(`
		SELECT ea.id, ea.subject_id, s.name, ea.mode, ea.score, ea.total_questions,
		       ea.correct_count, ea.started_at, ea.finished_at
		FROM exam_attempts ea
		JOIN subjects s ON s.id = ea.subject_id
		WHERE ea.id = ?
	`, id).Scan(&a.ID, &a.SubjectID, &a.SubjectName, &a.Mode, &a.Score,
		&a.TotalQuestions, &a.CorrectCount, &a.StartedAt, &a.FinishedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *AttemptService) RecordAnswer(attemptID, questionID int64, selectedAnswerID *int64) error {
	_, err := s.DB.Exec(
		"INSERT INTO attempt_answers (attempt_id, question_id, selected_answer_id) VALUES (?, ?, ?)",
		attemptID, questionID, selectedAnswerID,
	)
	return err
}

func (s *AttemptService) RecordAnswers(attemptID, questionID int64, selectedAnswerIDs []int64) error {
	if len(selectedAnswerIDs) == 0 {
		return s.RecordAnswer(attemptID, questionID, nil)
	}
	for _, aid := range selectedAnswerIDs {
		aid := aid
		if err := s.RecordAnswer(attemptID, questionID, &aid); err != nil {
			return err
		}
	}
	return nil
}

func (s *AttemptService) FinishAttempt(attemptID int64) (*db.ExamAttempt, error) {
	// Get distinct questions in this attempt
	qRows, err := s.DB.Query(
		"SELECT DISTINCT question_id FROM attempt_answers WHERE attempt_id = ?", attemptID)
	if err != nil {
		return nil, err
	}
	defer qRows.Close()

	var questionIDs []int64
	for qRows.Next() {
		var qid int64
		if err := qRows.Scan(&qid); err != nil {
			return nil, err
		}
		questionIDs = append(questionIDs, qid)
	}

	correctCount := 0
	for _, qid := range questionIDs {
		if correct, _ := s.isQuestionCorrect(attemptID, qid); correct {
			correctCount++
		}
	}

	totalQuestions := len(questionIDs)
	var score float64
	if totalQuestions > 0 {
		score = float64(correctCount) / float64(totalQuestions) * 100
	}

	now := time.Now()
	_, err = s.DB.Exec(`
		UPDATE exam_attempts
		SET correct_count = ?, total_questions = ?, score = ?, finished_at = ?
		WHERE id = ?
	`, correctCount, totalQuestions, score, now, attemptID)
	if err != nil {
		return nil, err
	}

	return s.GetAttempt(attemptID)
}

// isQuestionCorrect checks if the selected answers exactly match the correct answers
func (s *AttemptService) isQuestionCorrect(attemptID, questionID int64) (bool, error) {
	// Get selected answer IDs
	rows, err := s.DB.Query(
		"SELECT selected_answer_id FROM attempt_answers WHERE attempt_id = ? AND question_id = ? AND selected_answer_id IS NOT NULL",
		attemptID, questionID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	selectedIDs := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return false, err
		}
		selectedIDs[id] = true
	}

	// Get correct answer IDs
	correctRows, err := s.DB.Query(
		"SELECT id FROM answers WHERE question_id = ? AND is_correct = 1", questionID)
	if err != nil {
		return false, err
	}
	defer correctRows.Close()

	correctIDs := make(map[int64]bool)
	for correctRows.Next() {
		var id int64
		if err := correctRows.Scan(&id); err != nil {
			return false, err
		}
		correctIDs[id] = true
	}

	// Must match exactly: same count and all correct IDs selected
	if len(selectedIDs) != len(correctIDs) {
		return false, nil
	}
	for id := range correctIDs {
		if !selectedIDs[id] {
			return false, nil
		}
	}
	return true, nil
}

func (s *AttemptService) GetAttemptDetails(attemptID int64) ([]db.AttemptDetailRow, error) {
	// Get distinct questions in order
	qRows, err := s.DB.Query(`
		SELECT DISTINCT q.id, q.content, q.explanation, q.multi_answer
		FROM attempt_answers aa
		JOIN questions q ON q.id = aa.question_id
		WHERE aa.attempt_id = ?
		ORDER BY aa.id
	`, attemptID)
	if err != nil {
		return nil, err
	}
	defer qRows.Close()

	type questionInfo struct {
		id          int64
		content     string
		explanation string
		multiAnswer bool
	}
	var questions []questionInfo
	for qRows.Next() {
		var qi questionInfo
		if err := qRows.Scan(&qi.id, &qi.content, &qi.explanation, &qi.multiAnswer); err != nil {
			return nil, err
		}
		questions = append(questions, qi)
	}

	var details []db.AttemptDetailRow
	for _, qi := range questions {
		// Get selected answers
		selRows, err := s.DB.Query(`
			SELECT a.label, a.content
			FROM attempt_answers aa
			JOIN answers a ON a.id = aa.selected_answer_id
			WHERE aa.attempt_id = ? AND aa.question_id = ?
		`, attemptID, qi.id)
		if err != nil {
			return nil, err
		}
		var selectedLabels []string
		for selRows.Next() {
			var label, content string
			if err := selRows.Scan(&label, &content); err != nil {
				selRows.Close()
				return nil, err
			}
			selectedLabels = append(selectedLabels, label+". "+content)
		}
		selRows.Close()

		// Get correct answers
		corRows, err := s.DB.Query(`
			SELECT label, content FROM answers
			WHERE question_id = ? AND is_correct = 1
		`, qi.id)
		if err != nil {
			return nil, err
		}
		var correctLabels []string
		for corRows.Next() {
			var label, content string
			if err := corRows.Scan(&label, &content); err != nil {
				corRows.Close()
				return nil, err
			}
			correctLabels = append(correctLabels, label+". "+content)
		}
		corRows.Close()

		isCorrect, _ := s.isQuestionCorrect(attemptID, qi.id)

		details = append(details, db.AttemptDetailRow{
			QuestionContent: qi.content,
			Explanation:     qi.explanation,
			MultiAnswer:     qi.multiAnswer,
			SelectedLabels:  selectedLabels,
			CorrectLabels:   correctLabels,
			IsCorrect:       isCorrect,
		})
	}

	return details, nil
}

func (s *AttemptService) GetAnsweredQuestionIDs(attemptID int64) ([]int64, error) {
	rows, err := s.DB.Query("SELECT DISTINCT question_id FROM attempt_answers WHERE attempt_id = ?", attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *AttemptService) GetRecentAttempts(subjectID int64, limit int) ([]db.ExamAttempt, error) {
	rows, err := s.DB.Query(`
		SELECT ea.id, ea.subject_id, s.name, ea.mode, ea.score, ea.total_questions,
		       ea.correct_count, ea.started_at, ea.finished_at
		FROM exam_attempts ea
		JOIN subjects s ON s.id = ea.subject_id
		WHERE ea.subject_id = ? AND ea.finished_at IS NOT NULL AND ea.score > 0
		ORDER BY ea.finished_at DESC
		LIMIT ?
	`, subjectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []db.ExamAttempt
	for rows.Next() {
		var a db.ExamAttempt
		if err := rows.Scan(&a.ID, &a.SubjectID, &a.SubjectName, &a.Mode, &a.Score,
			&a.TotalQuestions, &a.CorrectCount, &a.StartedAt, &a.FinishedAt); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

func (s *AttemptService) GetSubjectStats(subjectID int64) (*db.SubjectStats, error) {
	var stats db.SubjectStats

	// Get average and best score
	err := s.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(AVG(score), 0), COALESCE(MAX(score), 0)
		FROM exam_attempts
		WHERE subject_id = ? AND finished_at IS NOT NULL AND score > 0
	`, subjectID).Scan(&stats.TotalAttempts, &stats.AvgScore, &stats.BestScore)
	if err != nil {
		return nil, err
	}

	recent, err := s.GetRecentAttempts(subjectID, 20)
	if err != nil {
		return nil, err
	}
	stats.RecentAttempts = recent

	return &stats, nil
}
