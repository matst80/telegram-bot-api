package tgbotapi

import (
	"strings"
	"testing"
)

func TestMD2V2(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "**bold**",
			expected: "*bold*",
		},
		{
			input:    "*italic*",
			expected: "_italic_",
		},
		{
			input:    "~~strike~~",
			expected: "~strike~",
		},
		{
			input:    "__underline__",
			expected: "__underline__",
		},
		{
			input:    "### Heading",
			expected: "*Heading*",
		},
		{
			input:    "[link](http://example.com/foo_bar)",
			expected: "[link](http://example.com/foo_bar)",
		},
		{
			input:    "> quote",
			expected: "> quote",
		},
		{
			input:    "**Rayleigh scattering** is the main reason (λ).",
			expected: "*Rayleigh scattering* is the main reason \\(λ\\)\\.",
		},
		{
			input:    "```python\nprint('hello')\n```",
			expected: "```python\nprint('hello')\n```",
		},
		{
			input:    "- list item",
			expected: "\\- list item",
		},
		{
			input:    "**bold *italic* bold**",
			expected: "*bold _italic_ bold*",
		},
		{
			input:    "Nested `inline code` in **bold**",
			expected: "Nested `inline code` in *bold*",
		},
		{
			input:    "Escape these: \\ _ * [ ] ( ) ~ ` > # + - = | { } . !",
			expected: "Escape these: \\\\ \\_ \\* \\[ \\] \\( \\) \\~ \\` \\> \\# \\+ \\- \\= \\| \\{ \\} \\. \\!",
		},
		{
			input:    "Link with underscore: [link](http://example.com/foo_bar)",
			expected: "Link with underscore: [link](http://example.com/foo_bar)",
		},
		{
			input:    "Code block with reserved: ```\nfoo_bar * baz\n```",
			expected: "Code block with reserved: ```\nfoo_bar * baz\n```",
		},
		{
			input:    "Incomplete ```code block",
			expected: "Incomplete \\`\\`\\`code block",
		},
		{
			input:    "Keyboard block should be stripped: ```keyboard\nbtn1 | btn2\n```",
			expected: "Keyboard block should be stripped: ",
		},
		{
			input:    "||hidden text||",
			expected: "||hidden text||",
		},
		{
			input:    "**||bold spoiler||**",
			expected: "*||bold spoiler||*",
		},
		{
			input:    "||**bold** and _italic_|| inside",
			expected: "||*bold* and _italic_|| inside",
		},
	}

	for _, tc := range tests {
		got := MD2V2(tc.input)
		if got != tc.expected {
			t.Errorf("MD2V2(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestComplexMD(t *testing.T) {
	input := `**Rayleigh scattering** is the main reason the sky appears blue during the day.

### Quick Explanation:
- **Sunlight** entering Earth's atmosphere is white light.
- **Air molecules** (mostly nitrogen and oxygen) are much smaller.

### Visuals:
- **Clear day**: Blue sky.
- **Sunset/sunrise**: Light travels through more atmosphere.

For a demo, check [NASA](https://www.nasa.gov/).`
	got := MD2V2(input)
	if !strings.Contains(got, "*Rayleigh scattering*") || !strings.Contains(got, "blue during the day\\.") {
		t.Errorf("Complex MD conversion failed, got: %s", got)
	}
}

func TestTransformers(t *testing.T) {
	state := &TransformState{}

	t.Run("KeyboardTransformer", func(t *testing.T) {
		tr := &KeyboardTransformer{}
		input := "text ```keyboard\nbtn\n``` end"
		got := tr.Transform(input, state)
		if got != "text  end" {
			t.Errorf("Expected 'text  end', got %q", got)
		}
	})

	t.Run("HeadingTransformer", func(t *testing.T) {
		tr := &HeadingTransformer{}
		got := tr.Transform("### Heading", state)
		if got != "**Heading**" {
			t.Errorf("Expected '**Heading**', got %q", got)
		}
	})

	t.Run("EscapeTransformer", func(t *testing.T) {
		tr := &EscapeTransformer{}
		got := tr.Transform("file_name.txt", state)
		if got != "file\\_name\\.txt" {
			t.Errorf("Expected escaped string, got %q", got)
		}
	})

	t.Run("QuoteTransformer", func(t *testing.T) {
		tr := &QuoteTransformer{}
		got := tr.Transform("> quote", state)
		if got != "> quote" {
			t.Errorf("Expected '> quote', got %q", got)
		}
	})
}
