package main

import (
	"strings"
	"testing"
)

func TestLinkifyText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "https link",
			raw:  "open https://example.com/path?q=1",
			want: `open <a href="https://example.com/path?q=1" target="_blank" rel="noopener noreferrer">https://example.com/path?q=1</a>`,
		},
		{
			name: "bare domain",
			raw:  "visit example.com now",
			want: `visit <a href="https://example.com" target="_blank" rel="noopener noreferrer">example.com</a> now`,
		},
		{
			name: "trailing punctuation",
			raw:  "see example.com.",
			want: `see <a href="https://example.com" target="_blank" rel="noopener noreferrer">example.com</a>.`,
		},
		{
			name: "escape surrounding text",
			raw:  `<script>alert(1)</script> example.com`,
			want: `&lt;script&gt;alert(1)&lt;/script&gt; <a href="https://example.com" target="_blank" rel="noopener noreferrer">example.com</a>`,
		},
		{
			name: "skip email domain",
			raw:  "mail a@example.com",
			want: `mail a@example.com`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := string(linkifyText(tt.raw))
			if got != tt.want {
				t.Fatalf("linkifyText(%q)\n got: %s\nwant: %s", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLinkifyTextOpensNewTabs(t *testing.T) {
	t.Parallel()

	got := string(linkifyText("example.com"))
	if !strings.Contains(got, `target="_blank"`) || !strings.Contains(got, `rel="noopener noreferrer"`) {
		t.Fatalf("link does not include safe new-tab attributes: %s", got)
	}
}
