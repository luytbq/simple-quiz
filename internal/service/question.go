package service

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"strings"

	"quiz/internal/db"
)

type QuestionService struct {
	DB *sql.DB
}

func (s *QuestionService) ListSubjects() ([]db.Subject, error) {
	rows, err := s.DB.Query(`
		SELECT s.id, s.name, s.description, s.share_code, s.created_at, COUNT(q.id)
		FROM subjects s
		LEFT JOIN questions q ON q.subject_id = s.id
		GROUP BY s.id
		ORDER BY s.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjects []db.Subject
	for rows.Next() {
		var sub db.Subject
		if err := rows.Scan(&sub.ID, &sub.Name, &sub.Description, &sub.ShareCode, &sub.CreatedAt, &sub.QuestionCount); err != nil {
			return nil, err
		}
		subjects = append(subjects, sub)
	}
	return subjects, rows.Err()
}

func (s *QuestionService) GetSubject(id int64) (*db.Subject, error) {
	var sub db.Subject
	err := s.DB.QueryRow(`
		SELECT s.id, s.name, s.description, s.share_code, s.created_at, COUNT(q.id)
		FROM subjects s
		LEFT JOIN questions q ON q.subject_id = s.id
		WHERE s.id = ?
		GROUP BY s.id
	`, id).Scan(&sub.ID, &sub.Name, &sub.Description, &sub.ShareCode, &sub.CreatedAt, &sub.QuestionCount)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// FindSubjectByName returns the subject with the given name (with its question
// count), or nil if no such subject exists.
func (s *QuestionService) FindSubjectByName(name string) (*db.Subject, error) {
	var sub db.Subject
	err := s.DB.QueryRow(`
		SELECT s.id, s.name, s.description, s.share_code, s.created_at, COUNT(q.id)
		FROM subjects s
		LEFT JOIN questions q ON q.subject_id = s.id
		WHERE s.name = ?
		GROUP BY s.id
	`, name).Scan(&sub.ID, &sub.Name, &sub.Description, &sub.ShareCode, &sub.CreatedAt, &sub.QuestionCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *QuestionService) GetSubjectByShareCode(code string) (*db.Subject, error) {
	var sub db.Subject
	err := s.DB.QueryRow(`
		SELECT s.id, s.name, s.description, s.share_code, s.created_at, COUNT(q.id)
		FROM subjects s
		LEFT JOIN questions q ON q.subject_id = s.id
		WHERE s.share_code = ?
		GROUP BY s.id
	`, code).Scan(&sub.ID, &sub.Name, &sub.Description, &sub.ShareCode, &sub.CreatedAt, &sub.QuestionCount)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *QuestionService) EnsureShareCode(subjectID int64) (string, error) {
	var code string
	err := s.DB.QueryRow("SELECT share_code FROM subjects WHERE id = ?", subjectID).Scan(&code)
	if err != nil {
		return "", err
	}
	if code != "" {
		return code, nil
	}
	code = generateShareCode()
	_, err = s.DB.Exec("UPDATE subjects SET share_code = ? WHERE id = ?", code, subjectID)
	return code, err
}

func generateShareCode() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

func (s *QuestionService) CountQuestions(subjectID int64) (int, error) {
	var count int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM questions WHERE subject_id = ?", subjectID).Scan(&count)
	return count, err
}

func (s *QuestionService) GetRandomQuestion(subjectID int64, excludeIDs []int64) (*db.Question, error) {
	query := "SELECT id, subject_id, content, explanation, multi_answer, order_number FROM questions WHERE subject_id = ?"
	args := []any{subjectID}

	if len(excludeIDs) > 0 {
		placeholders := make([]string, len(excludeIDs))
		for i, id := range excludeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += " AND id NOT IN (" + strings.Join(placeholders, ",") + ")"
	}

	query += " ORDER BY RANDOM() LIMIT 1"

	var q db.Question
	err := s.DB.QueryRow(query, args...).Scan(&q.ID, &q.SubjectID, &q.Content, &q.Explanation, &q.MultiAnswer, &q.OrderNumber)
	if err != nil {
		return nil, err
	}

	answers, err := s.getAnswers(q.ID)
	if err != nil {
		return nil, err
	}
	q.Answers = shuffleAnswers(answers)
	return &q, nil
}

func (s *QuestionService) GetQuestionWithAnswers(questionID int64) (*db.Question, error) {
	var q db.Question
	err := s.DB.QueryRow("SELECT id, subject_id, content, explanation, multi_answer, order_number FROM questions WHERE id = ?", questionID).
		Scan(&q.ID, &q.SubjectID, &q.Content, &q.Explanation, &q.MultiAnswer, &q.OrderNumber)
	if err != nil {
		return nil, err
	}

	answers, err := s.getAnswers(q.ID)
	if err != nil {
		return nil, err
	}
	q.Answers = shuffleAnswers(answers)
	return &q, nil
}

func (s *QuestionService) GetQuestions(subjectID int64, count int) ([]db.Question, error) {
	rows, err := s.DB.Query(
		"SELECT id, subject_id, content, explanation, multi_answer, order_number FROM questions WHERE subject_id = ? ORDER BY RANDOM() LIMIT ?",
		subjectID, count,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []db.Question
	for rows.Next() {
		var q db.Question
		if err := rows.Scan(&q.ID, &q.SubjectID, &q.Content, &q.Explanation, &q.MultiAnswer, &q.OrderNumber); err != nil {
			return nil, err
		}
		answers, err := s.getAnswers(q.ID)
		if err != nil {
			return nil, err
		}
		q.Answers = shuffleAnswers(answers)
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

func (s *QuestionService) GetCorrectAnswer(questionID int64) (*db.Answer, error) {
	var a db.Answer
	err := s.DB.QueryRow(
		"SELECT id, question_id, label, content, is_correct FROM answers WHERE question_id = ? AND is_correct = 1",
		questionID,
	).Scan(&a.ID, &a.QuestionID, &a.Label, &a.Content, &a.IsCorrect)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *QuestionService) GetCorrectAnswers(questionID int64) ([]db.Answer, error) {
	rows, err := s.DB.Query(
		"SELECT id, question_id, label, content, is_correct FROM answers WHERE question_id = ? AND is_correct = 1",
		questionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []db.Answer
	for rows.Next() {
		var a db.Answer
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.Label, &a.Content, &a.IsCorrect); err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, rows.Err()
}

func (s *QuestionService) GetAnswer(answerID int64) (*db.Answer, error) {
	var a db.Answer
	err := s.DB.QueryRow(
		"SELECT id, question_id, label, content, is_correct FROM answers WHERE id = ?",
		answerID,
	).Scan(&a.ID, &a.QuestionID, &a.Label, &a.Content, &a.IsCorrect)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *QuestionService) ImportQuestions(data db.ImportData) (*db.Subject, int, error) {
	normalizeChapters(&data)

	tx, err := s.DB.Begin()
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	// Upsert subject
	var subjectID int64
	err = tx.QueryRow("SELECT id FROM subjects WHERE name = ?", data.Subject).Scan(&subjectID)
	if err == sql.ErrNoRows {
		res, err := tx.Exec("INSERT INTO subjects (name, share_code) VALUES (?, ?)", data.Subject, generateShareCode())
		if err != nil {
			return nil, 0, fmt.Errorf("insert subject: %w", err)
		}
		subjectID, _ = res.LastInsertId()
	} else if err != nil {
		return nil, 0, err
	}

	count, err := insertChaptersAndQuestions(tx, subjectID, &data)
	if err != nil {
		return nil, 0, err
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}

	sub, err := s.GetSubject(subjectID)
	return sub, count, err
}

func (s *QuestionService) ReplaceSubject(subjectID int64, data db.ImportData) (*db.Subject, int, error) {
	normalizeChapters(&data)

	tx, err := s.DB.Begin()
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE subjects SET name = ? WHERE id = ?", data.Subject, subjectID); err != nil {
		return nil, 0, fmt.Errorf("update subject: %w", err)
	}

	// attempt_answers.question_id has no ON DELETE CASCADE, must delete manually
	if _, err := tx.Exec(`DELETE FROM attempt_answers WHERE question_id IN (SELECT id FROM questions WHERE subject_id = ?)`, subjectID); err != nil {
		return nil, 0, fmt.Errorf("delete attempt_answers: %w", err)
	}
	// Old chapter selections of past attempts reference chapters we are about to
	// drop. Delete them explicitly (also covered by ON DELETE CASCADE).
	if _, err := tx.Exec(`DELETE FROM attempt_chapters WHERE chapter_id IN (SELECT id FROM chapters WHERE subject_id = ?)`, subjectID); err != nil {
		return nil, 0, fmt.Errorf("delete attempt_chapters: %w", err)
	}
	// Questions reference chapters, so delete questions before chapters.
	if _, err := tx.Exec("DELETE FROM questions WHERE subject_id = ?", subjectID); err != nil {
		return nil, 0, fmt.Errorf("delete questions: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM chapters WHERE subject_id = ?", subjectID); err != nil {
		return nil, 0, fmt.Errorf("delete chapters: %w", err)
	}

	count, err := insertChaptersAndQuestions(tx, subjectID, &data)
	if err != nil {
		return nil, 0, err
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}

	sub, err := s.GetSubject(subjectID)
	return sub, count, err
}

func (s *QuestionService) DeleteSubject(id int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM attempt_answers WHERE attempt_id IN (SELECT id FROM exam_attempts WHERE subject_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM exam_attempts WHERE subject_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM subjects WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *QuestionService) ExportSubject(subjectID int64) (*db.ImportData, error) {
	sub, err := s.GetSubject(subjectID)
	if err != nil {
		return nil, err
	}

	chapters, err := s.ListChapters(subjectID)
	if err != nil {
		return nil, err
	}
	// Only emit chapter info when there is real grouping (>1 chapter). A single
	// chapter (e.g. the auto-created default) round-trips as the legacy format.
	includeChapters := len(chapters) > 1

	rows, err := s.DB.Query(
		"SELECT id, chapter_id, content, explanation, multi_answer FROM questions WHERE subject_id = ? ORDER BY order_number",
		subjectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []db.ImportQuestion
	for rows.Next() {
		var qID int64
		var chapterID *int64
		var content, explanation string
		var multiAnswer bool
		if err := rows.Scan(&qID, &chapterID, &content, &explanation, &multiAnswer); err != nil {
			return nil, err
		}

		answers, err := s.getAnswers(qID)
		if err != nil {
			return nil, err
		}

		var importAnswers []db.ImportAnswer
		for _, a := range answers {
			importAnswers = append(importAnswers, db.ImportAnswer{
				Label:     a.Label,
				Content:   a.Content,
				IsCorrect: a.IsCorrect,
			})
		}

		iq := db.ImportQuestion{
			Content: content,
			Answers: importAnswers,
		}
		if explanation != "" {
			iq.Explanation = explanation
		}
		if includeChapters && chapterID != nil {
			id := int(*chapterID)
			iq.ChapterID = &id
		}

		questions = append(questions, iq)
	}

	data := &db.ImportData{
		Subject:   sub.Name,
		Questions: questions,
	}
	if includeChapters {
		for _, c := range chapters {
			data.Chapters = append(data.Chapters, db.ImportChapter{
				ID:         int(c.ID),
				Name:       c.Name,
				Importance: c.Importance,
			})
		}
	}
	return data, nil
}

func (s *QuestionService) getAnswers(questionID int64) ([]db.Answer, error) {
	rows, err := s.DB.Query(
		"SELECT id, question_id, label, content, is_correct FROM answers WHERE question_id = ?",
		questionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []db.Answer
	for rows.Next() {
		var a db.Answer
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.Label, &a.Content, &a.IsCorrect); err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, rows.Err()
}

func shuffleAnswers(answers []db.Answer) []db.Answer {
	shuffled := make([]db.Answer, len(answers))
	copy(shuffled, answers)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled
}
