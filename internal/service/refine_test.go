package service

import (
	"strings"
	"testing"
)

func TestRefineImportData_ValidJSON(t *testing.T) {
	raw := `{"subject":"Math","questions":[{"content":"1+1?","answers":[{"label":"A","content":"2","is_correct":true},{"label":"B","content":"3","is_correct":false}]}]}`
	result := RefineImportData(raw)
	if !result.OK {
		t.Fatalf("expected OK=true, got false; errors=%v, help=%s", result.Errors, result.HelpHTML)
	}
	if result.Data.Subject != "Math" {
		t.Errorf("expected subject=Math, got %q", result.Data.Subject)
	}
	if len(result.Data.Questions) != 1 {
		t.Errorf("expected 1 question, got %d", len(result.Data.Questions))
	}
}

func TestRefineImportData_StripMarkdownWrapper(t *testing.T) {
	raw := "```json\n" + `{"subject":"Go","questions":[{"content":"Q?","answers":[{"label":"A","content":"yes","is_correct":true},{"label":"B","content":"no","is_correct":false}]}]}` + "\n```"
	result := RefineImportData(raw)
	if !result.OK {
		t.Fatalf("expected OK=true; errors=%v, help=%s", result.Errors, result.HelpHTML)
	}
	if result.Data.Subject != "Go" {
		t.Errorf("expected subject=Go, got %q", result.Data.Subject)
	}
	foundChange := false
	for _, c := range result.Changes {
		if strings.Contains(c, "markdown") || strings.Contains(c, "```") {
			foundChange = true
			break
		}
	}
	if !foundChange {
		t.Errorf("expected a change about stripping markdown wrapper, got %v", result.Changes)
	}
}

func TestRefineImportData_StripTrailingComma(t *testing.T) {
	raw := `{"subject":"X","questions":[{"content":"Q?","answers":[{"label":"A","content":"a","is_correct":true},{"label":"B","content":"b","is_correct":false},]}]}`
	result := RefineImportData(raw)
	if !result.OK {
		t.Fatalf("expected OK=true; errors=%v, help=%s", result.Errors, result.HelpHTML)
	}
	foundChange := false
	for _, c := range result.Changes {
		if strings.Contains(c, "trailing comma") {
			foundChange = true
			break
		}
	}
	if !foundChange {
		t.Errorf("expected trailing comma change, got %v", result.Changes)
	}
}

func TestRefineImportData_ExtractJSONFromExtraText(t *testing.T) {
	raw := `Here is the JSON: {"subject":"Test","questions":[{"content":"Q?","answers":[{"label":"A","content":"a","is_correct":true},{"label":"B","content":"b","is_correct":false}]}]} Hope this helps!`
	result := RefineImportData(raw)
	if !result.OK {
		t.Fatalf("expected OK=true; errors=%v, help=%s", result.Errors, result.HelpHTML)
	}
	if result.Data.Subject != "Test" {
		t.Errorf("expected subject=Test, got %q", result.Data.Subject)
	}
	foundChange := false
	for _, c := range result.Changes {
		if strings.Contains(c, "trích xuất") || strings.Contains(c, "extract") {
			foundChange = true
			break
		}
	}
	if !foundChange {
		t.Errorf("expected extraction change, got %v", result.Changes)
	}
}

func TestRefineImportData_FixSingleBacktickCodeBlocks(t *testing.T) {
	// Build valid JSON with content that has single backtick code blocks.
	// We use json.Marshal to avoid issues with literal newlines in JSON strings.
	raw := `{"subject":"Code","questions":[{"content":"Look at this:\n` + "`" + `java\nSystem.out.println();\n` + "`" + `","answers":[{"label":"A","content":"yes","is_correct":true},{"label":"B","content":"no","is_correct":false}]}]}`
	result := RefineImportData(raw)
	if !result.OK {
		t.Fatalf("expected OK=true; errors=%v, help=%s", result.Errors, result.HelpHTML)
	}
	content := result.Data.Questions[0].Content
	if !strings.Contains(content, "```java") {
		t.Errorf("expected triple backtick code block, got %q", content)
	}
}

func TestRefineImportData_InvalidJSON(t *testing.T) {
	raw := `{not valid json at all`
	result := RefineImportData(raw)
	if result.OK {
		t.Fatal("expected OK=false for invalid JSON")
	}
	if result.HelpHTML == "" {
		t.Error("expected HelpHTML to be set")
	}
	if !strings.Contains(result.HelpHTML, "JSON") {
		t.Errorf("expected help to mention JSON, got %q", result.HelpHTML)
	}
}

func TestRefineImportData_SchemaValidation(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		errMatch string
	}{
		{
			name:     "missing subject",
			raw:      `{"subject":"","questions":[{"content":"Q?","answers":[{"label":"A","content":"a","is_correct":true},{"label":"B","content":"b","is_correct":false}]}]}`,
			errMatch: "subject",
		},
		{
			name:     "missing questions",
			raw:      `{"subject":"X","questions":[]}`,
			errMatch: "questions",
		},
		{
			name:     "missing correct answer",
			raw:      `{"subject":"X","questions":[{"content":"Q?","answers":[{"label":"A","content":"a","is_correct":false},{"label":"B","content":"b","is_correct":false}]}]}`,
			errMatch: "is_correct",
		},
		{
			name:     "too few answers",
			raw:      `{"subject":"X","questions":[{"content":"Q?","answers":[{"label":"A","content":"a","is_correct":true}]}]}`,
			errMatch: "đáp án",
		},
		{
			name:     "empty content",
			raw:      `{"subject":"X","questions":[{"content":"","answers":[{"label":"A","content":"a","is_correct":true},{"label":"B","content":"b","is_correct":false}]}]}`,
			errMatch: "content",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RefineImportData(tc.raw)
			if result.OK {
				t.Fatal("expected OK=false")
			}
			if result.HelpHTML == "" {
				t.Fatal("expected HelpHTML to be set")
			}
			// Check that at least one error contains our match string
			found := false
			for _, e := range result.Errors {
				if strings.Contains(strings.ToLower(e), strings.ToLower(tc.errMatch)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected an error containing %q, got %v", tc.errMatch, result.Errors)
			}
		})
	}
}

func TestRefineImportData_HelpHTMLForSchemaErrors(t *testing.T) {
	raw := `{"subject":"","questions":[]}`
	result := RefineImportData(raw)
	if result.OK {
		t.Fatal("expected OK=false")
	}
	if !strings.Contains(result.HelpHTML, "<ul>") {
		t.Error("expected HelpHTML to contain <ul> for error list")
	}
	if !strings.Contains(result.HelpHTML, "<li>") {
		t.Error("expected HelpHTML to contain <li> for error items")
	}
}

func TestRefineImportData_HelpHTMLForInvalidJSON(t *testing.T) {
	raw := `{broken`
	result := RefineImportData(raw)
	if result.OK {
		t.Fatal("expected OK=false")
	}
	if !strings.Contains(result.HelpHTML, "<code>") {
		t.Error("expected HelpHTML to contain error in <code> tag")
	}
}

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantOK  bool
	}{
		{
			name:   "simple object",
			input:  `{"key":"value"}`,
			want:   `{"key":"value"}`,
			wantOK: true,
		},
		{
			name:   "object with prefix",
			input:  `prefix {"key":"value"} suffix`,
			want:   `{"key":"value"}`,
			wantOK: true,
		},
		{
			name:   "nested braces",
			input:  `{"a":{"b":"c"}}`,
			want:   `{"a":{"b":"c"}}`,
			wantOK: true,
		},
		{
			name:   "braces in string",
			input:  `{"a":"{}"}`,
			want:   `{"a":"{}"}`,
			wantOK: true,
		},
		{
			name:   "no object",
			input:  `no braces here`,
			want:   `no braces here`,
			wantOK: false,
		},
		{
			name:   "unbalanced braces",
			input:  `{"key":"value"`,
			want:   `{"key":"value"`,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractJSONObject(tc.input)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
