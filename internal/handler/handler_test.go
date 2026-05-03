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

func TestRenderMd(t *testing.T) {
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
			result := renderMd(tc.input)
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

func TestRenderMdi(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
		wantExcludes []string
		wantHasBlock bool // if true, result should NOT be just plain inline
	}{
		{
			name:         "strips wrapper for single paragraph",
			input:        "Hello",
			wantContains: []string{"Hello"},
			wantExcludes: []string{"<p>", "</p>"},
		},
		{
			name:         "inline code preserved, no p wrapper",
			input:        "Use `foo`",
			wantContains: []string{"<code>foo</code>"},
			wantExcludes: []string{"<p>", "</p>"},
		},
		{
			name:         "bold preserved, no p wrapper",
			input:        "**bold**",
			wantContains: []string{"<strong>bold</strong>"},
			wantExcludes: []string{"<p>", "</p>"},
		},
		{
			name:         "multi-paragraph keeps valid HTML",
			input:        "Line one\n\nLine two",
			wantContains: []string{"<p>Line one</p>", "<p>Line two</p>"},
		},
		{
			name:         "code block kept as block element",
			input:        "```go\nfmt.Println()\n```",
			wantContains: []string{"<pre>", "<code", "fmt.Println()", "</pre>"},
		},
		{
			name:         "list kept as block element",
			input:        "- first\n- second",
			wantContains: []string{"<ul>", "<li>first</li>", "<li>second</li>", "</ul>"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := renderMdi(tc.input)
			for _, s := range tc.wantContains {
				if !strings.Contains(result, s) {
					t.Errorf("expected result to contain %q, got %q", s, result)
				}
			}
			for _, s := range tc.wantExcludes {
				if strings.Contains(result, s) {
					t.Errorf("expected result NOT to contain %q, got %q", s, result)
				}
			}
		})
	}
}

func TestRenderMdi_ValidHTMLForMultiParagraph(t *testing.T) {
	// Previous implementation produced broken HTML like "Line 1</p>\n<p>Line 2"
	// by blindly stripping <p>/</p>. Verify the current implementation returns balanced tags.
	result := renderMdi("A\n\nB")
	openP := strings.Count(result, "<p>")
	closeP := strings.Count(result, "</p>")
	if openP != closeP {
		t.Errorf("unbalanced <p>/</p> tags: %d opens, %d closes in %q", openP, closeP, result)
	}
	if openP < 2 {
		t.Errorf("expected at least 2 paragraphs, got %q", result)
	}
}

func TestHasBlockTag(t *testing.T) {
	cases := map[string]bool{
		"plain text":                                    false,
		"<p>paragraph</p>":                              false,
		"<em>em</em>":                                   false,
		"<strong>bold</strong>":                         false,
		"<code>inline</code>":                           false,
		"<pre><code>block</code></pre>":                 true,
		"<ul><li>x</li></ul>":                           true,
		"<ol><li>x</li></ol>":                           true,
		"<table><tr><td>x</td></tr></table>":            true,
		"<blockquote>q</blockquote>":                    true,
		"<h1>t</h1>":                                    true,
		"<div class=\"mermaid\">graph TD; A-->B;</div>": true,
	}
	for input, want := range cases {
		if got := hasBlockTag(input); got != want {
			t.Errorf("hasBlockTag(%q) = %v, want %v", input, got, want)
		}
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
