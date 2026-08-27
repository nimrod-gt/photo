package model

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	KeywordCount   = 50
	MaxTitleLength = 200
)

type Tags struct {
	Title     string
	Keywords  []string
	Place     Place
	Concept   string
	Editorial Editorial
}

// Editorial is the mark stock sites ask for on a photo that may only be sold as
// editorial content, together with the day it was taken - the caption they want
// names it.
type Editorial struct {
	Marked bool
	Date   time.Time
}

// Normalized cuts the day out of the date and throws away everything an
// unmarked flag has no use for, so a value read back from a file compares equal
// to the one the dialog holds. A full timestamp never would - the calendar
// gives a local midnight, the file a parsed day - and every dialog close would
// rewrite the sidecar over a difference nothing shows.
func (e Editorial) Normalized() Editorial {
	if !e.Marked {
		return Editorial{}
	}
	if e.Date.IsZero() {
		return Editorial{Marked: true}
	}
	year, month, day := e.Date.Date()
	return Editorial{Marked: true, Date: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// A date without the mark is nothing to write: the day alone says neither that
// the photo is editorial nor that it is not.
func (e Editorial) IsEmpty() bool {
	return !e.Marked
}

// Place is the location the user typed in the Tags dialog plus the split the
// generator managed to make of it. A level it could not name stays empty rather
// than being guessed.
type Place struct {
	Location string
	City     string
	State    string
	Country  string
}

func (p Place) Trimmed() Place {
	return Place{
		Location: strings.TrimSpace(p.Location),
		City:     strings.TrimSpace(p.City),
		State:    strings.TrimSpace(p.State),
		Country:  strings.TrimSpace(p.Country),
	}
}

func (p Place) IsEmpty() bool {
	return p.Trimmed() == Place{}
}

func (t Tags) KeywordLine() string {
	return strings.Join(t.Keywords, ", ")
}

// Neither the place, the concept nor the editorial flag is looked at: callers
// read IsEmpty as "the generator produced nothing", and none of the three is
// something it produces - a place, a note or a mark alone must not pass for a
// result, or an empty run would overwrite a sidecar or blank the dialog.
func (t Tags) IsEmpty() bool {
	return len(strings.TrimSpace(t.Title)) == 0 && len(t.Keywords) == 0
}

func (t Tags) Equal(other Tags) bool {
	return t.Title == other.Title && t.Place == other.Place && t.Concept == other.Concept &&
		t.Editorial.Normalized() == other.Editorial.Normalized() &&
		slices.Equal(t.Keywords, other.Keywords)
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
	if bad := disallowed(t.Title); len(bad) != 0 {
		problems = append(problems, "title has disallowed characters: "+strings.Join(bad, " "))
	}
	if bad := disallowedKeywords(t.Keywords); len(bad) != 0 {
		problems = append(problems, "keywords have disallowed characters: "+strings.Join(bad, ", "))
	}
	if dupes := duplicateKeywords(t.Keywords); len(dupes) != 0 {
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
		if len(disallowed(keyword)) != 0 {
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
		if trimmed := strings.TrimSpace(part); len(trimmed) != 0 {
			keywords = append(keywords, trimmed)
		}
	}
	return keywords
}
