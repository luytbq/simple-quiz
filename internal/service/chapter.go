package service

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"

	"quiz/internal/db"
)

// clampImportance normalises a chapter importance to the valid 1..10 range,
// defaulting a missing/zero value to db.DefaultImportance.
func clampImportance(v int) int {
	if v == 0 {
		return db.DefaultImportance
	}
	if v < 1 {
		return 1
	}
	if v > 10 {
		return 10
	}
	return v
}

// normalizeChapters fills in sensible defaults for chapter-related fields so
// that import rarely fails on chapter problems. It is idempotent. Strategy:
// never error, always fill — a chapter missing a name gets its id as the name,
// importance is clamped to 1..10 (default 5), and any chapter_id referenced by
// a question but not declared is auto-created. Returns human-readable
// descriptions of the changes it made.
func normalizeChapters(data *db.ImportData) []string {
	var changes []string

	seen := make(map[int]bool)
	for i := range data.Chapters {
		ch := &data.Chapters[i]
		if ch.Name == "" {
			ch.Name = fmt.Sprintf("%d", ch.ID)
			changes = append(changes, fmt.Sprintf("Chương id=%d: thiếu \"name\", đặt name = \"%d\"", ch.ID, ch.ID))
		}
		if orig := ch.Importance; clampImportance(orig) != orig {
			ch.Importance = clampImportance(orig)
			changes = append(changes, fmt.Sprintf("Chương \"%s\": importance %d → %d", ch.Name, orig, ch.Importance))
		}
		seen[ch.ID] = true
	}

	// Auto-create chapters referenced by questions but not declared.
	for i := range data.Questions {
		q := &data.Questions[i]
		if q.ChapterID == nil {
			continue
		}
		id := *q.ChapterID
		if !seen[id] {
			data.Chapters = append(data.Chapters, db.ImportChapter{
				ID:         id,
				Name:       fmt.Sprintf("%d", id),
				Importance: db.DefaultImportance,
			})
			seen[id] = true
			changes = append(changes, fmt.Sprintf("Câu %d: chapter_id=%d chưa khai báo, tự tạo chương \"%d\"", i+1, id, id))
		}
	}

	return changes
}

// insertChaptersAndQuestions persists the chapters and questions of an import
// inside an existing transaction. It assumes normalizeChapters has already been
// applied. A default chapter ("Mặc định") is created lazily when any question
// has no chapter_id (or when no chapters are declared at all). Returns the
// number of questions inserted.
func insertChaptersAndQuestions(tx *sql.Tx, subjectID int64, data *db.ImportData) (int, error) {
	fileToDB := make(map[int]int64)
	order := 0
	for _, ch := range data.Chapters {
		if _, ok := fileToDB[ch.ID]; ok {
			continue // dedupe duplicate file ids, first wins
		}
		order++
		res, err := tx.Exec(
			"INSERT INTO chapters (subject_id, name, importance, order_number) VALUES (?, ?, ?, ?)",
			subjectID, ch.Name, clampImportance(ch.Importance), order,
		)
		if err != nil {
			return 0, fmt.Errorf("insert chapter %q: %w", ch.Name, err)
		}
		id, _ := res.LastInsertId()
		fileToDB[ch.ID] = id
	}

	var defaultChapterID int64
	getDefaultChapter := func() (int64, error) {
		if defaultChapterID != 0 {
			return defaultChapterID, nil
		}
		order++
		res, err := tx.Exec(
			"INSERT INTO chapters (subject_id, name, importance, order_number) VALUES (?, ?, ?, ?)",
			subjectID, db.DefaultChapterName, db.DefaultImportance, order,
		)
		if err != nil {
			return 0, fmt.Errorf("insert default chapter: %w", err)
		}
		defaultChapterID, _ = res.LastInsertId()
		return defaultChapterID, nil
	}

	count := 0
	for i, q := range data.Questions {
		var chapterID int64
		if q.ChapterID != nil {
			if id, ok := fileToDB[*q.ChapterID]; ok {
				chapterID = id
			}
		}
		if chapterID == 0 {
			id, err := getDefaultChapter()
			if err != nil {
				return 0, err
			}
			chapterID = id
		}

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
			"INSERT INTO questions (subject_id, chapter_id, content, explanation, multi_answer, order_number) VALUES (?, ?, ?, ?, ?, ?)",
			subjectID, chapterID, q.Content, q.Explanation, multiAnswer, i+1,
		)
		if err != nil {
			return 0, fmt.Errorf("insert question %d: %w", i+1, err)
		}
		qID, _ := res.LastInsertId()

		for _, a := range q.Answers {
			if _, err := tx.Exec(
				"INSERT INTO answers (question_id, label, content, is_correct) VALUES (?, ?, ?, ?)",
				qID, a.Label, a.Content, a.IsCorrect,
			); err != nil {
				return 0, fmt.Errorf("insert answer for question %d: %w", i+1, err)
			}
		}
		count++
	}

	return count, nil
}

// ListChapters returns the chapters of a subject ordered by order_number, each
// with its current question count.
func (s *QuestionService) ListChapters(subjectID int64) ([]db.Chapter, error) {
	rows, err := s.DB.Query(`
		SELECT c.id, c.subject_id, c.name, c.importance, c.order_number, c.created_at, COUNT(q.id)
		FROM chapters c
		LEFT JOIN questions q ON q.chapter_id = c.id
		WHERE c.subject_id = ?
		GROUP BY c.id
		ORDER BY c.order_number, c.id
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chapters []db.Chapter
	for rows.Next() {
		var c db.Chapter
		if err := rows.Scan(&c.ID, &c.SubjectID, &c.Name, &c.Importance, &c.OrderNumber, &c.CreatedAt, &c.QuestionCount); err != nil {
			return nil, err
		}
		chapters = append(chapters, c)
	}
	return chapters, rows.Err()
}

// CountQuestionsInChapters counts questions in the given chapters. An empty
// chapterIDs slice means "all chapters of the subject".
func (s *QuestionService) CountQuestionsInChapters(subjectID int64, chapterIDs []int64) (int, error) {
	query := "SELECT COUNT(*) FROM questions WHERE subject_id = ?"
	args := []any{subjectID}
	if len(chapterIDs) > 0 {
		query += " AND chapter_id IN (" + placeholders(len(chapterIDs)) + ")"
		for _, id := range chapterIDs {
			args = append(args, id)
		}
	}
	var count int
	err := s.DB.QueryRow(query, args...).Scan(&count)
	return count, err
}

// GetRandomQuestionInChapters returns a random question from the given chapters
// excluding the supplied ids. An empty chapterIDs slice means "all chapters".
func (s *QuestionService) GetRandomQuestionInChapters(subjectID int64, chapterIDs, excludeIDs []int64) (*db.Question, error) {
	query := "SELECT id, subject_id, chapter_id, content, explanation, multi_answer, order_number FROM questions WHERE subject_id = ?"
	args := []any{subjectID}

	if len(chapterIDs) > 0 {
		query += " AND chapter_id IN (" + placeholders(len(chapterIDs)) + ")"
		for _, id := range chapterIDs {
			args = append(args, id)
		}
	}
	if len(excludeIDs) > 0 {
		query += " AND id NOT IN (" + placeholders(len(excludeIDs)) + ")"
		for _, id := range excludeIDs {
			args = append(args, id)
		}
	}
	query += " ORDER BY RANDOM() LIMIT 1"

	var q db.Question
	err := s.DB.QueryRow(query, args...).Scan(&q.ID, &q.SubjectID, &q.ChapterID, &q.Content, &q.Explanation, &q.MultiAnswer, &q.OrderNumber)
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

// GetExamQuestions builds an exam paper of `count` questions drawn from the
// given chapters (empty = all chapters), allocating the count across chapters by
// importance via allocateQuestions. The returned questions are shuffled so the
// chapters are interleaved.
func (s *QuestionService) GetExamQuestions(subjectID int64, chapterIDs []int64, count int) ([]db.Question, error) {
	allocs, err := s.chapterAllocs(subjectID, chapterIDs)
	if err != nil {
		return nil, err
	}
	if len(allocs) == 0 {
		return nil, nil
	}

	perChapter := allocateQuestions(allocs, count)

	var questions []db.Question
	for _, a := range allocs {
		n := perChapter[a.ID]
		if n <= 0 {
			continue
		}
		rows, err := s.DB.Query(
			"SELECT id, subject_id, chapter_id, content, explanation, multi_answer, order_number FROM questions WHERE chapter_id = ? ORDER BY RANDOM() LIMIT ?",
			a.ID, n,
		)
		if err != nil {
			return nil, err
		}
		var batch []db.Question
		for rows.Next() {
			var q db.Question
			if err := rows.Scan(&q.ID, &q.SubjectID, &q.ChapterID, &q.Content, &q.Explanation, &q.MultiAnswer, &q.OrderNumber); err != nil {
				rows.Close()
				return nil, err
			}
			batch = append(batch, q)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for i := range batch {
			answers, err := s.getAnswers(batch[i].ID)
			if err != nil {
				return nil, err
			}
			batch[i].Answers = shuffleAnswers(answers)
		}
		questions = append(questions, batch...)
	}

	rand.Shuffle(len(questions), func(i, j int) {
		questions[i], questions[j] = questions[j], questions[i]
	})
	return questions, nil
}

// chapterAllocs loads the importance and available question count for the given
// chapters (empty = all chapters of the subject). Chapters with no questions are
// skipped — they cannot contribute to an exam.
func (s *QuestionService) chapterAllocs(subjectID int64, chapterIDs []int64) ([]chapterAlloc, error) {
	query := `
		SELECT c.id, c.importance, COUNT(q.id)
		FROM chapters c
		LEFT JOIN questions q ON q.chapter_id = c.id
		WHERE c.subject_id = ?`
	args := []any{subjectID}
	if len(chapterIDs) > 0 {
		query += " AND c.id IN (" + placeholders(len(chapterIDs)) + ")"
		for _, id := range chapterIDs {
			args = append(args, id)
		}
	}
	query += " GROUP BY c.id ORDER BY c.order_number, c.id"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocs []chapterAlloc
	for rows.Next() {
		var a chapterAlloc
		if err := rows.Scan(&a.ID, &a.Importance, &a.Available); err != nil {
			return nil, err
		}
		if a.Available > 0 {
			allocs = append(allocs, a)
		}
	}
	return allocs, rows.Err()
}

// chapterAlloc is the input to allocateQuestions: a chapter's importance weight
// and how many questions it actually has available.
type chapterAlloc struct {
	ID         int64
	Importance int
	Available  int
}

// allocateQuestions distributes `target` questions across chapters by
// importance. Guarantees, in priority order matching the spec:
//  3. sum(result) == min(target, total available)            — exact total
//  2. every chapter gets >= 1 when target >= number of chapters — min one each
//  1. the remainder is distributed proportionally to importance  — proportional
//
// Each chapter is also capped at its Available count. When target is smaller
// than the number of chapters, the most important chapters are kept (criterion 3
// wins over criterion 2).
func allocateQuestions(chapters []chapterAlloc, target int) map[int64]int {
	result := make(map[int64]int, len(chapters))
	for _, c := range chapters {
		result[c.ID] = 0
	}

	total := 0
	weightSum := 0
	for _, c := range chapters {
		total += c.Available
		weightSum += c.Importance
	}
	n := target
	if n > total {
		n = total
	}
	if n <= 0 || len(chapters) == 0 {
		return result
	}

	// Priority order: importance desc, then more available, then stable by id.
	order := make([]chapterAlloc, len(chapters))
	copy(order, chapters)
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].Importance != order[j].Importance {
			return order[i].Importance > order[j].Importance
		}
		if order[i].Available != order[j].Available {
			return order[i].Available > order[j].Available
		}
		return order[i].ID < order[j].ID
	})

	// Phase 1 — give each chapter at least one, prioritised, within budget.
	placed := 0
	for _, c := range order {
		if placed == n {
			break
		}
		if c.Available >= 1 {
			result[c.ID] = 1
			placed++
		}
	}

	// Phase 2 — distribute the remainder by largest proportional deficit.
	for placed < n {
		bestID := int64(-1)
		bestDeficit := 0.0
		bestImportance := -1
		for _, c := range order {
			if result[c.ID] >= c.Available {
				continue // at capacity
			}
			var ideal float64
			if weightSum > 0 {
				ideal = float64(n) * float64(c.Importance) / float64(weightSum)
			} else {
				ideal = float64(n) / float64(len(chapters))
			}
			deficit := ideal - float64(result[c.ID])
			if bestID == -1 || deficit > bestDeficit ||
				(deficit == bestDeficit && c.Importance > bestImportance) {
				bestID = c.ID
				bestDeficit = deficit
				bestImportance = c.Importance
			}
		}
		if bestID == -1 {
			break // no capacity left (shouldn't happen since n <= total)
		}
		result[bestID]++
		placed++
	}

	return result
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
