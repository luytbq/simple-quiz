package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"quiz/internal/db"
)

// RefineResult holds the result of refining import data
type RefineResult struct {
	Data     db.ImportData
	Changes  []string // list of changes made
	Errors   []string // validation errors
	HelpHTML string   // HTML help message if failed
	OK       bool     // true if ready to import
}

// RefineImportData attempts to fix and validate raw input text
func RefineImportData(raw string) *RefineResult {
	result := &RefineResult{}

	// Step 1: Extract and fix JSON
	jsonStr, changes := fixJSON(raw)
	result.Changes = append(result.Changes, changes...)

	// Step 2: Try parse
	var data db.ImportData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		result.HelpHTML = fmt.Sprintf(
			`<p><strong>Dữ liệu không phải JSON hợp lệ.</strong></p>
			<p>Lỗi: <code>%s</code></p>
			<p>Hãy kiểm tra lại nội dung. Đảm bảo dữ liệu là JSON hợp lệ, bắt đầu bằng <code>{</code> và kết thúc bằng <code>}</code>.</p>`,
			escapeHTML(err.Error()),
		)
		return result
	}

	// Step 3: Fix rich content
	for i := range data.Questions {
		qChanges := fixRichContent(&data.Questions[i], i+1)
		result.Changes = append(result.Changes, qChanges...)
	}

	// Step 4: Schema validation
	errors := validateSchema(&data)
	if len(errors) > 0 {
		result.Errors = errors
		result.Data = data

		var helpLines []string
		helpLines = append(helpLines, "<p><strong>Dữ liệu không hợp lệ:</strong></p><ul>")
		for _, e := range errors {
			helpLines = append(helpLines, fmt.Sprintf("<li>%s</li>", escapeHTML(e)))
		}
		helpLines = append(helpLines, "</ul>")
		result.HelpHTML = strings.Join(helpLines, "\n")
		return result
	}

	result.Data = data
	result.OK = true
	return result
}

// fixJSON attempts to extract and fix JSON from raw text
func fixJSON(raw string) (string, []string) {
	var changes []string
	s := strings.TrimSpace(raw)

	// Strip markdown ```json wrapper
	if strings.HasPrefix(s, "```") {
		// Find closing ```
		firstNewline := strings.Index(s, "\n")
		lastFence := strings.LastIndex(s, "```")
		if firstNewline > 0 && lastFence > firstNewline {
			s = strings.TrimSpace(s[firstNewline+1 : lastFence])
			changes = append(changes, "Đã bỏ markdown wrapper ```json...```")
		}
	}

	// Try parse as-is first
	if json.Valid([]byte(s)) {
		return s, changes
	}

	// Strip trailing commas before ] or }
	cleaned := trailingCommaRe.ReplaceAllString(s, "$1")
	if cleaned != s {
		s = cleaned
		changes = append(changes, "Đã bỏ trailing comma thừa")
	}
	if json.Valid([]byte(s)) {
		return s, changes
	}

	// Try to extract JSON object by brace balancing
	extracted, ok := extractJSONObject(s)
	if ok && extracted != s {
		changes = append(changes, "Đã trích xuất JSON từ text thừa")
		s = extracted

		// Try fix trailing commas on extracted
		cleaned = trailingCommaRe.ReplaceAllString(s, "$1")
		if cleaned != s {
			s = cleaned
			changes = append(changes, "Đã bỏ trailing comma thừa")
		}
	}

	return s, changes
}

var trailingCommaRe = regexp.MustCompile(`,\s*([}\]])`)

// extractJSONObject finds outermost {...} by brace balancing
func extractJSONObject(s string) (string, bool) {
	start := -1
	depth := 0
	inString := false
	escaped := false

	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			if start == -1 {
				start = i
			}
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 && start >= 0 {
				return s[start : i+1], true
			}
		}
	}
	return s, false
}

// Known language names for code block detection
var knownLanguages = []string{
	"java", "python", "go", "javascript", "typescript", "sql",
	"c", "cpp", "csharp", "ruby", "php", "swift", "kotlin",
	"rust", "scala", "html", "css", "bash", "shell", "json",
	"xml", "yaml", "mermaid",
}

// singleBacktickCodeRe matches `lang\ncode\n` (single backtick code blocks)
var singleBacktickCodeRe = regexp.MustCompile(
	"(?s)`(" + strings.Join(knownLanguages, "|") + ")\\n(.*?)`",
)

// fixRichContent fixes markdown formatting issues in a question
func fixRichContent(q *db.ImportQuestion, num int) []string {
	var changes []string

	// Fix question content
	if fixed, desc := fixContentMarkdown(q.Content, num, "câu hỏi"); fixed != q.Content {
		q.Content = fixed
		changes = append(changes, desc...)
	}

	// Fix explanation
	if fixed, desc := fixContentMarkdown(q.Explanation, num, "giải thích"); fixed != q.Explanation {
		q.Explanation = fixed
		changes = append(changes, desc...)
	}

	// Fix answers
	for j := range q.Answers {
		if fixed, desc := fixContentMarkdown(q.Answers[j].Content, num, fmt.Sprintf("đáp án %s", q.Answers[j].Label)); fixed != q.Answers[j].Content {
			q.Answers[j].Content = fixed
			changes = append(changes, desc...)
		}
	}

	return changes
}

func fixContentMarkdown(content string, qNum int, field string) (string, []string) {
	if content == "" {
		return content, nil
	}

	var changes []string
	result := content

	// Fix single backtick code blocks → triple backtick
	if singleBacktickCodeRe.MatchString(result) {
		result = singleBacktickCodeRe.ReplaceAllStringFunc(result, func(match string) string {
			parts := singleBacktickCodeRe.FindStringSubmatch(match)
			if len(parts) < 3 {
				return match
			}
			return "```" + parts[1] + "\n" + parts[2] + "```"
		})
		if result != content {
			changes = append(changes, fmt.Sprintf("Câu %d (%s): Sửa single backtick → triple backtick code block", qNum, field))
		}
	}

	// Ensure newlines around code blocks
	// Add \n\n before ``` if not present
	re := regexp.MustCompile("([^\n])(\n```)")
	if re.MatchString(result) {
		result = re.ReplaceAllString(result, "$1\n$2")
		if result != content {
			changes = append(changes, fmt.Sprintf("Câu %d (%s): Thêm dòng trống trước code block", qNum, field))
		}
	}

	return result, changes
}

// validateSchema validates the import data against expected schema
func validateSchema(data *db.ImportData) []string {
	var errors []string

	if data.Subject == "" {
		errors = append(errors, "Thiếu trường \"subject\" — tên chủ đề không được để trống")
	}

	if len(data.Questions) == 0 {
		errors = append(errors, "Thiếu trường \"questions\" — cần ít nhất 1 câu hỏi")
		return errors
	}

	for i, q := range data.Questions {
		num := i + 1

		if q.Content == "" {
			errors = append(errors, fmt.Sprintf("Câu %d: thiếu nội dung câu hỏi (\"content\")", num))
		}

		if len(q.Answers) < 2 {
			errors = append(errors, fmt.Sprintf("Câu %d: chỉ có %d đáp án, cần ít nhất 2", num, len(q.Answers)))
			continue
		}

		hasCorrect := false
		for j, a := range q.Answers {
			if a.Label == "" {
				errors = append(errors, fmt.Sprintf("Câu %d, đáp án %d: thiếu \"label\"", num, j+1))
			}
			if a.Content == "" {
				errors = append(errors, fmt.Sprintf("Câu %d, đáp án %s: thiếu nội dung", num, a.Label))
			}
			if a.IsCorrect {
				hasCorrect = true
			}
		}

		if !hasCorrect {
			errors = append(errors, fmt.Sprintf("Câu %d: không có đáp án nào được đánh dấu \"is_correct\": true", num))
		}
	}

	return errors
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
