package tags

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"photo/internal/core/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRun struct {
	name   string
	args   []string
	output string
	err    error
}

func (f *fakeRun) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.name = name
	f.args = args
	return []byte(f.output), f.err
}

func (f *fakeRun) argValue(flag string) string {
	if i := slices.Index(f.args, flag); i >= 0 && i+1 < len(f.args) {
		return f.args[i+1]
	}
	return ""
}

func newTestTagger(run *fakeRun) *Tagger {
	return &Tagger{
		run:     run.run,
		lookup:  func(string) (string, error) { return "/usr/local/bin/claude", nil },
		timeout: time.Minute,
	}
}

func testRequest(notes string) Request {
	return Request{
		Photo: model.Photo{ImagePath: filepath.Join("/photos", "trip", "DSC1.JPG"), Name: "DSC1.JPG"},
		Notes: notes,
	}
}

func TestTagger_Generate(t *testing.T) {
	t.Parallel()

	t.Run("returns tags from structured output", func(t *testing.T) {
		run := &fakeRun{output: `{"is_error":false,"result":"done",
			"structured_output":{"title":"Sunset over the bay","keywords":["sunset","bay"]}}`}

		tags, err := newTestTagger(run).Generate(t.Context(), testRequest(""))
		require.NoError(t, err)
		assert.Equal(t, "Sunset over the bay", tags.Title)
		assert.Equal(t, []string{"sunset", "bay"}, tags.Keywords)
	})

	t.Run("builds the command line", func(t *testing.T) {
		run := &fakeRun{output: `{"structured_output":{"title":"t","keywords":["k"]}}`}

		_, err := newTestTagger(run).Generate(t.Context(), testRequest(""))
		require.NoError(t, err)

		assert.Equal(t, "/usr/local/bin/claude", run.name)
		assert.Equal(t, "Photo: /photos/trip/DSC1.JPG", run.argValue("-p"))
		assert.Equal(t, "/photos/trip", run.argValue("--add-dir"))
		assert.Equal(t, "Read", run.argValue("--allowedTools"))
		assert.Equal(t, "json", run.argValue("--output-format"))
		assert.Equal(t, taggerModel, run.argValue("--model"))
		assert.JSONEq(t, tagsSchema, run.argValue("--json-schema"))
		assert.Equal(t, stockPhotoPrompt, run.argValue("--system-prompt"))
		assert.Contains(t, run.args, "--safe-mode")
		assert.Contains(t, run.args, "--no-session-persistence")
	})

	t.Run("appends notes to the user message", func(t *testing.T) {
		run := &fakeRun{output: `{"structured_output":{"title":"t","keywords":["k"]}}`}

		_, err := newTestTagger(run).Generate(t.Context(), testRequest("  Location: Lisbon\nConcept: travel  "))
		require.NoError(t, err)
		assert.Equal(t, "Photo: /photos/trip/DSC1.JPG\nLocation: Lisbon\nConcept: travel", run.argValue("-p"))
	})

	t.Run("blank notes leave the message unchanged", func(t *testing.T) {
		run := &fakeRun{output: `{"structured_output":{"title":"t","keywords":["k"]}}`}

		_, err := newTestTagger(run).Generate(t.Context(), testRequest("   \n  "))
		require.NoError(t, err)
		assert.Equal(t, "Photo: /photos/trip/DSC1.JPG", run.argValue("-p"))
	})

	t.Run("propagates the lookup failure", func(t *testing.T) {
		tagger := &Tagger{
			run:     (&fakeRun{}).run,
			lookup:  func(string) (string, error) { return "", ErrClaudeNotFound },
			timeout: time.Minute,
		}

		_, err := tagger.Generate(t.Context(), testRequest(""))
		assert.ErrorIs(t, err, ErrClaudeNotFound)
	})

	t.Run("passes the configured path to the lookup", func(t *testing.T) {
		var got string
		run := &fakeRun{output: `{"structured_output":{"title":"t","keywords":["k"]}}`}
		tagger := &Tagger{
			run: run.run,
			lookup: func(configured string) (string, error) {
				got = configured
				return configured, nil
			},
			timeout: time.Minute,
		}

		req := testRequest("")
		req.ClaudePath = "/opt/claude"
		_, err := tagger.Generate(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, "/opt/claude", got)
		assert.Equal(t, "/opt/claude", run.name)
	})

	t.Run("wraps the run failure", func(t *testing.T) {
		failure := errors.New("exit status 1")
		run := &fakeRun{err: failure}

		_, err := newTestTagger(run).Generate(t.Context(), testRequest(""))
		assert.ErrorIs(t, err, failure)
	})

	t.Run("applies the timeout", func(t *testing.T) {
		tagger := &Tagger{
			run: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
			lookup:  func(string) (string, error) { return "/usr/local/bin/claude", nil },
			timeout: 10 * time.Millisecond,
		}

		_, err := tagger.Generate(t.Context(), testRequest(""))
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("keeps a response without keywords", func(t *testing.T) {
		run := &fakeRun{output: `{"structured_output":{"title":"Only a title"}}`}

		tags, err := newTestTagger(run).Generate(t.Context(), testRequest(""))
		require.NoError(t, err)
		assert.Equal(t, "Only a title", tags.Title)
		assert.Empty(t, tags.Keywords)
		assert.Contains(t, tags.Problems(), "0 keywords, expected 50")
	})
}

func TestParseTagsResponse(t *testing.T) {
	t.Parallel()

	t.Run("malformed JSON", func(t *testing.T) {
		_, err := parseTagsResponse([]byte("not json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not json")
	})

	t.Run("is_error surfaces the result text", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{"is_error":true,"result":"credit balance too low"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "credit balance too low")
	})

	t.Run("missing structured output", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{"is_error":false,"result":"I cannot read that file"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "I cannot read that file")
	})

	t.Run("empty response", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty response")
	})
}

func TestExcerpt(t *testing.T) {
	t.Parallel()

	t.Run("trims whitespace", func(t *testing.T) {
		assert.Equal(t, "hello", excerpt([]byte("  hello\n")))
	})

	t.Run("truncates long text", func(t *testing.T) {
		long := make([]byte, excerptLimit+50)
		for i := range long {
			long[i] = 'x'
		}
		got := excerpt(long)
		assert.Len(t, got, excerptLimit+3)
		assert.Less(t, len(got), len(long))
	})
}

func TestStockPhotoPrompt(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, stockPhotoPrompt)
	assert.Contains(t, stockPhotoPrompt, "keywords")
	for _, key := range []string{"Photo:", "Concept:", "Location:", "Editorial:"} {
		assert.Contains(t, stockPhotoPrompt, key, "the prompt must document every request key the dialog sends")
	}
}

func TestRunClaude_CancelKillsTheProcess(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == osWindows {
		t.Skip("no sleep binary on windows")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runClaude(ctx, "sleep", "30")
	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Contains(t, err.Error(), "timed out")
}
