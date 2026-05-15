package main

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

var linkPattern = regexp.MustCompile(`(?i)\b(?:https?://[^\s<>"']+|(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?::\d{2,5})?(?:/[^\s<>"']*)?)`)

func linkifyText(value string) template.HTML {
	if value == "" {
		return ""
	}

	var b strings.Builder
	last := 0
	for _, match := range linkPattern.FindAllStringIndex(value, -1) {
		start, end := match[0], match[1]
		if shouldSkipLink(value, start) {
			continue
		}

		raw := value[start:end]
		link, trailing := splitTrailingLinkPunctuation(raw)
		if link == "" {
			continue
		}

		b.WriteString(html.EscapeString(value[last:start]))

		href := link
		if !hasWebScheme(href) {
			href = "https://" + href
		}

		b.WriteString(`<a href="`)
		b.WriteString(html.EscapeString(href))
		b.WriteString(`" target="_blank" rel="noopener noreferrer">`)
		b.WriteString(html.EscapeString(link))
		b.WriteString(`</a>`)
		b.WriteString(html.EscapeString(trailing))

		last = end
	}

	b.WriteString(html.EscapeString(value[last:]))
	return template.HTML(b.String())
}

func shouldSkipLink(value string, start int) bool {
	if start <= 0 {
		return false
	}

	previous := value[start-1]
	return previous == '@' || previous == '/'
}

func splitTrailingLinkPunctuation(value string) (string, string) {
	end := len(value)
	for end > 0 && strings.ContainsRune(".,!?:;)]}", rune(value[end-1])) {
		end--
	}
	return value[:end], value[end:]
}

func hasWebScheme(value string) bool {
	value = strings.ToLower(value)
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
