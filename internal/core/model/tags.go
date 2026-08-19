package model

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const (
	KeywordCount   = 50
	MaxTitleLength = 200
)

type Tags struct {
	Title    string
	Keywords []string
}

func (t Tags) KeywordLine() string {
	return strings.Join(t.Keywords, ", ")
}

// Problems reports every stock requirement the tags violate, so the user can fix
// them by hand before saving. An empty result means the tags are ready to upload.
func (t Tags) Problems() []string {
	var problems []string

	if len(strings.TrimSpace(t.Title)) == 0 {
		problems = append(problems, "title is empty")
	}
	if length := len([]rune(t.Title)); length > MaxTitleLength {
		problems = append(problems, fmt.Sprintf("title is %d characters, limit is %d", length, MaxTitleLength))
	}
	if len(t.Keywords) != KeywordCount {
		problems = append(problems, fmt.Sprintf("%d keywords, expected %d", len(t.Keywords), KeywordCount))
	}
	if bad := disallowed(t.Title); len(bad) > 0 {
		problems = append(problems, "title has disallowed characters: "+strings.Join(bad, " "))
	}
	if bad := disallowedKeywords(t.Keywords); len(bad) > 0 {
		problems = append(problems, "keywords have disallowed characters: "+strings.Join(bad, ", "))
	}
	if dupes := duplicateKeywords(t.Keywords); len(dupes) > 0 {
		problems = append(problems, "duplicate keywords: "+strings.Join(dupes, ", "))
	}

	return problems
}

// disallowed returns the distinct characters outside the set the stock prompt
// allows: basic latin letters, digits, space, comma, period and hyphen.
// Non-printable characters are quoted so the message stays readable.
func disallowed(s string) []string {
	var found []string
	seen := make(map[rune]bool)
	for _, r := range s {
		if isAllowed(r) || seen[r] {
			continue
		}
		seen[r] = true
		found = append(found, displayRune(r))
	}
	return found
}

func displayRune(r rune) string {
	if unicode.IsPrint(r) {
		return string(r)
	}
	return strconv.QuoteRune(r)
}

func isAllowed(r rune) bool {
	if r > unicode.MaxASCII {
		return false
	}
	switch r {
	case ' ', ',', '.', '-':
		return true
	}
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func disallowedKeywords(keywords []string) []string {
	var bad []string
	for _, keyword := range keywords {
		if len(disallowed(keyword)) > 0 {
			bad = append(bad, keyword)
		}
	}
	return bad
}

func duplicateKeywords(keywords []string) []string {
	seen := make(map[string]bool, len(keywords))
	reported := make(map[string]bool)
	var dupes []string
	for _, keyword := range keywords {
		normalized := strings.ToLower(strings.TrimSpace(keyword))
		if len(normalized) == 0 {
			continue
		}
		if seen[normalized] {
			if !reported[normalized] {
				reported[normalized] = true
				dupes = append(dupes, normalized)
			}
			continue
		}
		seen[normalized] = true
	}
	return dupes
}

func ParseKeywordLine(line string) []string {
	var keywords []string
	for part := range strings.SplitSeq(line, ",") {
		if trimmed := strings.TrimSpace(part); len(trimmed) > 0 {
			keywords = append(keywords, trimmed)
		}
	}
	return keywords
}
