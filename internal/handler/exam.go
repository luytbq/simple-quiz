package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) ExamSetup(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	subject, err := h.Questions.GetSubject(subjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, "exam_setup.html", map[string]any{
		"Subject": subject,
	})
}

func (h *Handler) ExamStart(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	count, err := strconv.Atoi(r.FormValue("question_count"))
	if err != nil || count <= 0 {
		http.Error(w, "Invalid question count", http.StatusBadRequest)
		return
	}

	attempt, err := h.Attempts.CreateAttempt(subjectID, "exam", count)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.url(fmt.Sprintf("/exam/%d/take", attempt.ID)), http.StatusSeeOther)
}

func (h *Handler) ExamTake(w http.ResponseWriter, r *http.Request) {
	attemptID, err := pathInt64(r, "attemptID")
	if err != nil {
		http.Error(w, "Invalid attempt ID", http.StatusBadRequest)
		return
	}

	attempt, err := h.Attempts.GetAttempt(attemptID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	questions, err := h.Questions.GetQuestions(attempt.SubjectID, attempt.TotalQuestions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	subject, _ := h.Questions.GetSubject(attempt.SubjectID)

	h.render(w, "exam.html", map[string]any{
		"Subject":   subject,
		"Attempt":   attempt,
		"Questions": questions,
	})
}

func (h *Handler) ExamSubmit(w http.ResponseWriter, r *http.Request) {
	attemptID, err := pathInt64(r, "attemptID")
	if err != nil {
		http.Error(w, "Invalid attempt ID", http.StatusBadRequest)
		return
	}

	r.ParseForm()

	// Extract question IDs from form keys (q_{questionID})
	seen := make(map[int64]bool)
	for key := range r.Form {
		if !strings.HasPrefix(key, "q_") {
			continue
		}
		qid, err := strconv.ParseInt(strings.TrimPrefix(key, "q_"), 10, 64)
		if err != nil || seen[qid] {
			continue
		}
		seen[qid] = true

		values := r.Form[key]
		if len(values) == 0 {
			h.Attempts.RecordAnswer(attemptID, qid, nil)
			continue
		}
		var answerIDs []int64
		for _, v := range values {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				answerIDs = append(answerIDs, id)
			}
		}
		h.Attempts.RecordAnswers(attemptID, qid, answerIDs)
	}

	h.Attempts.FinishAttempt(attemptID)

	http.Redirect(w, r, h.url(fmt.Sprintf("/exam/%d/result", attemptID)), http.StatusSeeOther)
}

func (h *Handler) ExamResult(w http.ResponseWriter, r *http.Request) {
	attemptID, err := pathInt64(r, "attemptID")
	if err != nil {
		http.Error(w, "Invalid attempt ID", http.StatusBadRequest)
		return
	}

	attempt, err := h.Attempts.GetAttempt(attemptID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	details, err := h.Attempts.GetAttemptDetails(attemptID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	subject, _ := h.Questions.GetSubject(attempt.SubjectID)

	h.render(w, "exam_result.html", map[string]any{
		"Subject": subject,
		"Attempt": attempt,
		"Details": details,
	})
}
