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
		SELECT s.id, s.name, s.description, s.created_at, COUNT(q.id)
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
		if err := rows.Scan(&sub.ID, &sub.Name, &sub.Description, &sub.CreatedAt, &sub.QuestionCount); err != nil {
			return nil, err
		}
		subjects = append(subjects, sub)
	}
	return subjects, rows.Err()
}

func (s *QuestionService) GetSubject(id int64) (*db.Subject, error) {
	var sub db.Subject
	err := s.DB.QueryRow(`
		SELECT s.id, s.name, s.description, s.created_at, COUNT(q.id)
		FROM subjects s
		LEFT JOIN questions q ON q.subject_id = s.id
		WHERE s.id = ?
		GROUP BY s.id
	`, id).Scan(&sub.ID, &sub.Name, &sub.Description, &sub.CreatedAt, &sub.QuestionCount)
	if err != nil {
		return nil, err
	}
	return &sub, nil
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
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	// Upsert subject
	var subjectID int64
	err = tx.QueryRow("SELECT id FROM subjects WHERE name = ?", data.Subject).Scan(&subjectID)
	if err == sql.ErrNoRows {
		res, err := tx.Exec("INSERT INTO subjects (name) VALUES (?)", data.Subject)
		if err != nil {
			return nil, 0, fmt.Errorf("insert subject: %w", err)
		}
		subjectID, _ = res.LastInsertId()
	} else if err != nil {
		return nil, 0, err
	}

	count := 0
	for i, q := range data.Questions {
		// Auto-detect multi_answer if not explicitly set
		multiAnswer := false
		if q.MultiAnswer != nil {
			multiAnswer = *q.MultiAnswer
		} else {
			correctCount := 0
			for _, a := range q.Answers {
				if a.IsCorrect {
					correctCount++
				}
			}
			multiAnswer = correctCount > 1
		}

		res, err := tx.Exec(
			"INSERT INTO questions (subject_id, content, explanation, multi_answer, order_number) VALUES (?, ?, ?, ?, ?)",
			subjectID, q.Content, q.Explanation, multiAnswer, i+1,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("insert question %d: %w", i+1, err)
		}
		qID, _ := res.LastInsertId()

		for _, a := range q.Answers {
			_, err := tx.Exec(
				"INSERT INTO answers (question_id, label, content, is_correct) VALUES (?, ?, ?, ?)",
				qID, a.Label, a.Content, a.IsCorrect,
			)
			if err != nil {
				return nil, 0, fmt.Errorf("insert answer for question %d: %w", i+1, err)
			}
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}

	sub, err := s.GetSubject(subjectID)
	return sub, count, err
}

func (s *QuestionService) DeleteSubject(id int64) error {
	_, err := s.DB.Exec("DELETE FROM subjects WHERE id = ?", id)
	return err
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
