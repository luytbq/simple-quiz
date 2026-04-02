package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"quiz/internal/db"
)

func (h *Handler) ImportForm(w http.ResponseWriter, r *http.Request) {
	subjects, _ := h.Questions.ListSubjects()

	// Ensure all subjects have share codes
	for _, s := range subjects {
		if s.ShareCode == "" {
			h.Questions.EnsureShareCode(s.ID)
		}
	}
	if len(subjects) > 0 {
		// Re-fetch to get updated share codes
		subjects, _ = h.Questions.ListSubjects()
	}

	h.render(w, "import.html", map[string]any{
		"Subjects": subjects,
		"Error":    r.URL.Query().Get("error"),
		"Success":  r.URL.Query().Get("success"),
	})
}

func (h *Handler) ImportSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	jsonText := r.FormValue("json_data")

	if jsonText == "" {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("JSON data is required")), http.StatusSeeOther)
		return
	}

	var importData db.ImportData
	if err := json.Unmarshal([]byte(jsonText), &importData); err != nil {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Invalid JSON: "+err.Error())), http.StatusSeeOther)
		return
	}

	if importData.Subject == "" {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Missing 'subject' field")), http.StatusSeeOther)
		return
	}

	if len(importData.Questions) == 0 {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("No questions found")), http.StatusSeeOther)
		return
	}

	sub, count, err := h.Questions.ImportQuestions(importData)
	if err != nil {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Import failed: "+err.Error())), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Đã import %d câu hỏi cho '%s'", count, sub.Name)
	http.Redirect(w, r, h.url("/manage?success="+url.QueryEscape(msg)), http.StatusSeeOther)
}

func (h *Handler) DeleteSubject(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	sub, err := h.Questions.GetSubject(subjectID)
	if err != nil {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Subject not found")), http.StatusSeeOther)
		return
	}

	if err := h.Questions.DeleteSubject(subjectID); err != nil {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Delete failed: "+err.Error())), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Đã xoá '%s'", sub.Name)
	http.Redirect(w, r, h.url("/manage?success="+url.QueryEscape(msg)), http.StatusSeeOther)
}

func (h *Handler) ExportSubject(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	data, err := h.Questions.ExportSubject(subjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("%s.json", data.Subject)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(jsonBytes)
}
