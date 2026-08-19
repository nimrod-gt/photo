package model

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func validKeywords() []string {
	keywords := make([]string, 0, KeywordCount)
	for i := range KeywordCount {
		keywords = append(keywords, "keyword"+strconv.Itoa(i))
	}
	return keywords
}

func TestTags_Problems(t *testing.T) {
	t.Parallel()

	t.Run("valid tags", func(t *testing.T) {
		tags := Tags{Title: "Man walks along a beach. Travel and tourism concept.", Keywords: validKeywords()}
		assert.Empty(t, tags.Problems())
	})

	t.Run("empty title", func(t *testing.T) {
		tags := Tags{Title: "", Keywords: validKeywords()}
		assert.Contains(t, tags.Problems(), "title is empty")
	})

	t.Run("whitespace only title", func(t *testing.T) {
		tags := Tags{Title: "  \n ", Keywords: validKeywords()}
		assert.Contains(t, tags.Problems(), "title is empty")
	})

	t.Run("title too long", func(t *testing.T) {
		tags := Tags{Title: strings.Repeat("a", MaxTitleLength+1), Keywords: validKeywords()}
		assert.Contains(t, tags.Problems(), "title is 201 characters, limit is 200")
	})

	t.Run("title at the limit", func(t *testing.T) {
		tags := Tags{Title: strings.Repeat("a", MaxTitleLength), Keywords: validKeywords()}
		assert.Empty(t, tags.Problems())
	})

	t.Run("wrong keyword count", func(t *testing.T) {
		tags := Tags{Title: "Title", Keywords: []string{"one", "two"}}
		assert.Contains(t, tags.Problems(), "2 keywords, expected 50")
	})

	t.Run("disallowed characters in title", func(t *testing.T) {
		tags := Tags{Title: "Praia de Sao Martinho — Portugal", Keywords: validKeywords()}
		assert.Contains(t, tags.Problems(), "title has disallowed characters: —")
	})

	t.Run("disallowed characters in keyword", func(t *testing.T) {
		keywords := validKeywords()
		keywords[0] = "cascaisï"
		tags := Tags{Title: "Title", Keywords: keywords}
		assert.Contains(t, tags.Problems(), "keywords have disallowed characters: cascaisï")
	})

	t.Run("keyword repeated three times is reported once", func(t *testing.T) {
		keywords := validKeywords()
		keywords[1] = "keyword0"
		keywords[2] = "keyword0"
		tags := Tags{Title: "Title", Keywords: keywords}
		assert.Contains(t, tags.Problems(), "duplicate keywords: keyword0")
	})

	t.Run("duplicate keywords ignoring case", func(t *testing.T) {
		keywords := validKeywords()
		keywords[1] = "Keyword0"
		tags := Tags{Title: "Title", Keywords: keywords}
		assert.Contains(t, tags.Problems(), "duplicate keywords: keyword0")
	})

	t.Run("reports every problem at once", func(t *testing.T) {
		tags := Tags{Title: "", Keywords: []string{"one", "one"}}
		assert.Equal(t, []string{
			"title is empty",
			"2 keywords, expected 50",
			"duplicate keywords: one",
		}, tags.Problems())
	})
}

func TestDisallowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "plain ascii", input: "Man walks along a beach, 2026 - travel.", want: nil},
		{name: "diacritics", input: "São Martinho", want: []string{"ã"}},
		{name: "typographic dash and quotes", input: "beach — “summer”", want: []string{"—", "“", "”"}},
		{name: "repeated character reported once", input: "Zürich München", want: []string{"ü"}},
		{name: "forbidden ascii punctuation", input: "don't (stop) & go", want: []string{"'", "(", ")", "&"}},
		{name: "cyrillic", input: "пляж", want: []string{"п", "л", "я", "ж"}},
		{name: "non-printable quoted", input: "beach\tocean", want: []string{`'\t'`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, disallowed(tt.input))
		})
	}
}

func TestParseKeywordLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty line", input: "", want: nil},
		{name: "single keyword", input: "beach", want: []string{"beach"}},
		{name: "trims spaces", input: " beach ,  ocean,sand ", want: []string{"beach", "ocean", "sand"}},
		{name: "drops empty parts", input: "beach,,ocean,", want: []string{"beach", "ocean"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseKeywordLine(tt.input))
		})
	}
}

func TestTags_KeywordLine(t *testing.T) {
	t.Parallel()

	tags := Tags{Title: "Title", Keywords: []string{"beach", "ocean", "sand"}}
	assert.Equal(t, "beach, ocean, sand", tags.KeywordLine())
	assert.Equal(t, tags.Keywords, ParseKeywordLine(tags.KeywordLine()))
}

func TestTags_IsEmpty(t *testing.T) {
	t.Parallel()

	assert.True(t, Tags{}.IsEmpty())
	assert.True(t, Tags{Title: "   "}.IsEmpty())
	assert.False(t, Tags{Title: "A title."}.IsEmpty())
	assert.False(t, Tags{Keywords: []string{"lake"}}.IsEmpty())
}

func TestTags_Equal(t *testing.T) {
	t.Parallel()

	tags := Tags{Title: "A title.", Keywords: []string{"lake", "fog"}}

	assert.True(t, tags.Equal(Tags{Title: "A title.", Keywords: []string{"lake", "fog"}}))
	assert.False(t, tags.Equal(Tags{Title: "Another title.", Keywords: []string{"lake", "fog"}}))
	assert.False(t, tags.Equal(Tags{Title: "A title.", Keywords: []string{"lake"}}))
	assert.False(t, tags.Equal(Tags{Title: "A title.", Keywords: []string{"fog", "lake"}}))
	assert.True(t, Tags{}.Equal(Tags{Keywords: nil}))
}
