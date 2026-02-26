package slack

import (
	"testing"
)

func TestMrkdwnFormatter_Format(t *testing.T) {
	f := NewMrkdwnFormatter()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Bold conversion",
			input: "This is **bold** and __bold__ text.",
			want:  "This is *bold* and *bold* text.",
		},
		{
			name:  "Italic conversion",
			input: "This is *italic* and _italic_ text.",
			want:  "This is _italic_ and _italic_ text.",
		},
		{
			name:  "Strikethrough conversion",
			input: "This is ~~strikethrough~~ text.",
			want:  "This is ~strikethrough~ text.",
		},
		{
			name:  "Link conversion",
			input: "Check [HotPlex](https://hotplex.ai).",
			want:  "Check <https://hotplex.ai|HotPlex>.",
		},
		{
			name:  "Escaping special chars",
			input: "Tokens < 100 & Cost > 0",
			want:  "Tokens &lt; 100 &amp; Cost &gt; 0",
		},
		{
			name:  "Code block protection",
			input: "Here is code: ```fmt.Println(\"**no bold**\")```",
			want:  "Here is code: ```fmt.Println(\"**no bold**\")```",
		},
		{
			name:  "Mixed formatting",
			input: "**Bold** and _italic_ with a [link](http://example.com).",
			want:  "*Bold* and _italic_ with a <http://example.com|link>.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.Format(tt.input); got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}
