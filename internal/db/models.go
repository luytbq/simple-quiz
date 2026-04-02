package db

import "time"

type Subject struct {
	ID            int64
	Name          string
	Description   string
	ShareCode     string
	CreatedAt     time.Time
	QuestionCount int // populated by queries, not stored
}

type Question struct {
	ID          int64
	SubjectID   int64
	Content     string
	Explanation string
	MultiAnswer bool
	OrderNumber int
	CreatedAt   time.Time
	Answers     []Answer
}

type Answer struct {
	ID         int64
	QuestionID int64
	Label      string
	Content    string
	IsCorrect  bool
}

type ExamAttempt struct {
	ID             int64
	SubjectID      int64
	SubjectName    string // populated by joins
	Mode           string // "flashcard" or "exam"
	Score          float64
	TotalQuestions int
	CorrectCount   int
	StartedAt      time.Time
	FinishedAt     *time.Time
}

type AttemptAnswer struct {
	ID               int64
	AttemptID        int64
	QuestionID       int64
	SelectedAnswerID *int64
}

// AttemptDetailRow is used for exam review
type AttemptDetailRow struct {
	QuestionContent string
	Explanation     string
	MultiAnswer     bool
	SelectedLabels  []string // multiple selected answers
	CorrectLabels   []string // multiple correct answers
	IsCorrect       bool
}

// SubjectStats holds aggregated statistics for a subject
type SubjectStats struct {
	Subject        Subject
	TotalAttempts  int
	AvgScore       float64
	BestScore      float64
	RecentAttempts []ExamAttempt
}

// Import format structs
type ImportData struct {
	Subject   string           `json:"subject"`
	Questions []ImportQuestion `json:"questions"`
}

type ImportQuestion struct {
	Content     string         `json:"content"`
	Explanation string         `json:"explanation,omitempty"`
	MultiAnswer *bool          `json:"multi_answer,omitempty"` // auto-detected if not set
	Answers     []ImportAnswer `json:"answers"`
}

type ImportAnswer struct {
	Label     string `json:"label"`
	Content   string `json:"content"`
	IsCorrect bool   `json:"is_correct"`
}
