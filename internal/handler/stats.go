package handler

import "net/http"

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	subjects, err := h.Questions.ListSubjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type subjectWithStats struct {
		ID            int64
		Name          string
		QuestionCount int
		TotalAttempts int
		AvgScore      float64
	}

	var items []subjectWithStats
	for _, s := range subjects {
		stats, err := h.Attempts.GetSubjectStats(s.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items = append(items, subjectWithStats{
			ID:            s.ID,
			Name:          s.Name,
			QuestionCount: s.QuestionCount,
			TotalAttempts: stats.TotalAttempts,
			AvgScore:      stats.AvgScore,
		})
	}

	h.render(w, "stats.html", map[string]any{
		"Subjects": items,
	})
}

func (h *Handler) SubjectStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.Attempts.GetSubjectStats(subjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stats.Subject = *subject

	h.render(w, "stats_detail.html", map[string]any{
		"Stats": stats,
	})
}
