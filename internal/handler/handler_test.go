package handler

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConvertMermaidBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no mermaid block",
			input:    "<p>Hello world</p>",
			expected: "<p>Hello world</p>",
		},
		{
			name:     "single mermaid block",
			input:    `<pre><code class="language-mermaid">graph TD; A-->B;</code></pre>`,
			expected: `<div class="mermaid">graph TD; A-->B;</div>`,
		},
		{
			name:     "mermaid block with surrounding content",
			input:    `<p>Before</p><pre><code class="language-mermaid">graph LR; A-->B;</code></pre><p>After</p>`,
			expected: `<p>Before</p><div class="mermaid">graph LR; A-->B;</div><p>After</p>`,
		},
		{
			name:     "multiple mermaid blocks",
			input:    `<pre><code class="language-mermaid">graph TD;</code></pre> text <pre><code class="language-mermaid">sequenceDiagram</code></pre>`,
			expected: `<div class="mermaid">graph TD;</div> text <div class="mermaid">sequenceDiagram</div>`,
		},
		{
			name:     "non-mermaid code block unchanged",
			input:    `<pre><code class="language-go">fmt.Println("hi")</code></pre>`,
			expected: `<pre><code class="language-go">fmt.Println("hi")</code></pre>`,
		},
		{
			name:     "mermaid with newlines",
			input:    "<pre><code class=\"language-mermaid\">graph TD;\n  A-->B;\n  B-->C;</code></pre>",
			expected: "<div class=\"mermaid\">graph TD;\n  A-->B;\n  B-->C;</div>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := convertMermaidBlocks(tc.input)
			if got != tc.expected {
				t.Errorf("convertMermaidBlocks(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestMdTemplateFunc(t *testing.T) {
	mdFunc := func(s string) template.HTML {
		var buf strings.Builder
		mdRenderer.Convert([]byte(s), &buf)
		safe := sanitizePolicy.SanitizeBytes([]byte(buf.String()))
		result := convertMermaidBlocks(string(safe))
		return template.HTML(result)
	}

	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "plain text becomes paragraph",
			input:    "Hello world",
			contains: []string{"<p>", "Hello world", "</p>"},
		},
		{
			name:     "bold text",
			input:    "**bold**",
			contains: []string{"<strong>bold</strong>"},
		},
		{
			name:     "code block",
			input:    "```go\nfmt.Println()\n```",
			contains: []string{"<pre>", "<code", "fmt.Println()"},
		},
		{
			name:     "inline code",
			input:    "Use `foo` here",
			contains: []string{"<code>foo</code>"},
		},
		{
			name:     "script tag sanitized",
			input:    "<script>alert('xss')</script>",
			contains: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := string(mdFunc(tc.input))
			for _, s := range tc.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected result to contain %q, got %q", s, result)
				}
			}
			if tc.name == "script tag sanitized" {
				if strings.Contains(result, "<script>") {
					t.Errorf("expected <script> to be sanitized, got %q", result)
				}
			}
		})
	}
}

func TestMdiTemplateFunc(t *testing.T) {
	mdiFunc := func(s string) template.HTML {
		var buf strings.Builder
		mdRenderer.Convert([]byte(s), &buf)
		safe := sanitizePolicy.SanitizeBytes([]byte(buf.String()))
		str := string(safe)
		str = strings.TrimSpace(str)
		str = strings.TrimPrefix(str, "<p>")
		str = strings.TrimSuffix(str, "</p>")
		return template.HTML(str)
	}

	tests := []struct {
		name        string
		input       string
		shouldNotHave string
		shouldHave    string
	}{
		{
			name:          "strips p wrapper",
			input:         "Hello",
			shouldNotHave: "<p>",
			shouldHave:    "Hello",
		},
		{
			name:          "inline code preserved",
			input:         "Use `foo`",
			shouldNotHave: "<p>",
			shouldHave:    "<code>foo</code>",
		},
		{
			name:          "bold preserved",
			input:         "**bold**",
			shouldNotHave: "<p>",
			shouldHave:    "<strong>bold</strong>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := string(mdiFunc(tc.input))
			if tc.shouldNotHave != "" && strings.HasPrefix(result, tc.shouldNotHave) {
				t.Errorf("expected result NOT to start with %q, got %q", tc.shouldNotHave, result)
			}
			if !strings.Contains(result, tc.shouldHave) {
				t.Errorf("expected result to contain %q, got %q", tc.shouldHave, result)
			}
		})
	}
}

func TestCSPHeaderIsSet(t *testing.T) {
	// We can test the render method by checking that the CSP header is set.
	// Since render requires templates, we'll create a minimal handler with a test template.
	funcMap := template.FuncMap{
		"add":     func(a, b int) int { return a + b },
		"percent": func(score float64) string { return "" },
		"seq":     func(n int) []int { return nil },
		"bp":      func() string { return "" },
		"v":       func() string { return "" },
		"md":      func(s string) template.HTML { return "" },
		"mdi":     func(s string) template.HTML { return "" },
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).Parse(
		`{{define "layout"}}OK{{end}}`,
	))

	h := &Handler{
		templates: map[string]*template.Template{
			"test.html": tmpl,
		},
	}

	w := httptest.NewRecorder()
	h.render(w, "test.html", nil)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected Content-Security-Policy header to be set")
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("expected CSP to contain \"default-src 'self'\", got %q", csp)
	}
	if !strings.Contains(csp, "script-src") {
		t.Errorf("expected CSP to contain script-src directive, got %q", csp)
	}
	if !strings.Contains(csp, "style-src") {
		t.Errorf("expected CSP to contain style-src directive, got %q", csp)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCSPHeader_MissingTemplate(t *testing.T) {
	h := &Handler{
		templates: map[string]*template.Template{},
	}

	w := httptest.NewRecorder()
	h.render(w, "nonexistent.html", nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing template, got %d", w.Code)
	}
}
