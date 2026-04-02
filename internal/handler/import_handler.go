package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"quiz/internal/db"
)

func (h *Handler) ImportForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, "import.html", map[string]any{
		"Error":   r.URL.Query().Get("error"),
		"Success": r.URL.Query().Get("success"),
	})
}

func (h *Handler) ImportSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	jsonText := r.FormValue("json_data")

	if jsonText == "" {
		http.Redirect(w, r, "/import?error="+url.QueryEscape("JSON data is required"), http.StatusSeeOther)
		return
	}

	var importData db.ImportData
	if err := json.Unmarshal([]byte(jsonText), &importData); err != nil {
		http.Redirect(w, r, "/import?error="+url.QueryEscape("Invalid JSON: "+err.Error()), http.StatusSeeOther)
		return
	}

	if importData.Subject == "" {
		http.Redirect(w, r, "/import?error="+url.QueryEscape("Missing 'subject' field"), http.StatusSeeOther)
		return
	}

	if len(importData.Questions) == 0 {
		http.Redirect(w, r, "/import?error="+url.QueryEscape("No questions found"), http.StatusSeeOther)
		return
	}

	sub, count, err := h.Questions.ImportQuestions(importData)
	if err != nil {
		http.Redirect(w, r, "/import?error="+url.QueryEscape("Import failed: "+err.Error()), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Imported %d questions for '%s'", count, sub.Name)
	http.Redirect(w, r, "/?imported="+url.QueryEscape(msg), http.StatusSeeOther)
}
