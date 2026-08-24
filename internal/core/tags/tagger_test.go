package tags

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"photo/internal/core/claudebin"
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

// Reset times are rendered in the machine's own zone, so the tests live there too.
//
//nolint:gosmopolitan // deliberate: the parser formats local times
var testNow = time.Date(2026, time.August, 19, 9, 30, 0, 0, time.Local)

func newTestTagger(run *fakeRun) *Tagger {
	return &Tagger{
		run:     run.run,
		lookup:  func(string) (string, error) { return "/usr/local/bin/claude", nil },
		now:     func() time.Time { return testNow },
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
			lookup:  func(string) (string, error) { return "", claudebin.ErrNotFound },
			now:     func() time.Time { return testNow },
			timeout: time.Minute,
		}

		_, err := tagger.Generate(t.Context(), testRequest(""))
		assert.ErrorIs(t, err, claudebin.ErrNotFound)
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
			now:     func() time.Time { return testNow },
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

	t.Run("applies the timeout and names it", func(t *testing.T) {
		tagger := &Tagger{
			run: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
			lookup:  func(string) (string, error) { return "/usr/local/bin/claude", nil },
			now:     func() time.Time { return testNow },
			timeout: 10 * time.Millisecond,
		}

		_, err := tagger.Generate(t.Context(), testRequest(""))
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Contains(t, err.Error(), "claude timed out after 10ms")
	})

	t.Run("reads the report of a run that exited non-zero", func(t *testing.T) {
		run := &fakeRun{
			output: `{"is_error":true,"subtype":"error_during_execution","api_error_status":429,"result":"Overloaded"}`,
			err:    errors.New("exit status 1"),
		}

		_, err := newTestTagger(run).Generate(t.Context(), testRequest(""))
		require.Error(t, err)
		assert.Equal(t, "claude reported an error, error_during_execution, HTTP 429: Overloaded", err.Error())
	})

	t.Run("keeps tags that came with a non-zero exit", func(t *testing.T) {
		run := &fakeRun{
			output: `{"structured_output":{"title":"Sunset","keywords":["sunset"]}}`,
			err:    errors.New("exit status 1"),
		}

		tags, err := newTestTagger(run).Generate(t.Context(), testRequest(""))
		require.NoError(t, err)
		assert.Equal(t, "Sunset", tags.Title)
	})

	t.Run("falls back to the exit status when stdout is not a report", func(t *testing.T) {
		failure := errors.New("exit status 127")
		run := &fakeRun{output: "command not found", err: failure}

		_, err := newTestTagger(run).Generate(t.Context(), testRequest(""))
		require.ErrorIs(t, err, failure)
		assert.Contains(t, err.Error(), "running claude")
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
		_, err := parseTagsResponse([]byte("not json"), testNow)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not json")
	})

	t.Run("is_error surfaces the result text", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{"is_error":true,"result":"credit balance too low"}`), testNow)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "credit balance too low")
	})

	t.Run("missing structured output", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{"is_error":false,"result":"I cannot read that file"}`), testNow)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "I cannot read that file")
	})

	// Taken as a result it would blank the dialog and clear the sidecar.
	t.Run("an empty structured output is a failure", func(t *testing.T) {
		for _, output := range []string{
			`{"structured_output":{"title":"","keywords":[]}}`,
			`{"structured_output":{"title":"   ","keywords":null},"result":"nothing to describe"}`,
		} {
			_, err := parseTagsResponse([]byte(output), testNow)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "claude returned no tags")
		}
	})

	t.Run("names the failing stage and the HTTP status", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{"is_error":true,"subtype":"error_during_execution",
			"api_error_status":429,"result":"Overloaded"}`), testNow)
		require.Error(t, err)
		assert.Equal(t, "claude reported an error, error_during_execution, HTTP 429: Overloaded", err.Error())
	})

	t.Run("keeps a silent failure readable", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{"is_error":true,"subtype":"error_max_turns","result":""}`), testNow)
		require.Error(t, err)
		assert.Equal(t, "claude reported an error, error_max_turns: empty response", err.Error())
	})

	t.Run("spells out when the subscription limit resets", func(t *testing.T) {
		reset := testNow.Add(90 * time.Minute)
		out := fmt.Appendf(nil, `{"is_error":true,"result":"Claude usage limit reached|%d"}`, reset.Unix())

		_, err := parseTagsResponse(out, testNow)

		require.Error(t, err)
		assert.Equal(t, "claude reported an error: Claude usage limit reached, resets at 11:00", err.Error())
	})

	t.Run("dates a limit that resets on another day", func(t *testing.T) {
		reset := testNow.AddDate(0, 0, 2)
		out := fmt.Appendf(nil, `{"is_error":true,"result":"Claude usage limit reached|%d"}`, reset.Unix())

		_, err := parseTagsResponse(out, testNow)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "resets at Aug 21, 09:30")
	})

	t.Run("keeps a limit without a reset time as it came", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{"is_error":true,"result":"Usage limit reached"}`), testNow)
		require.Error(t, err)
		assert.Equal(t, "claude reported an error: Usage limit reached", err.Error())
	})

	t.Run("keeps a pipe inside the limit message", func(t *testing.T) {
		reset := testNow.Add(90 * time.Minute)
		out := fmt.Appendf(nil, `{"is_error":true,"result":"Claude usage limit reached (plan a|b)|%d"}`, reset.Unix())

		_, err := parseTagsResponse(out, testNow)

		require.Error(t, err)
		assert.Equal(t, "claude reported an error: Claude usage limit reached (plan a|b), resets at 11:00", err.Error())
	})

	t.Run("caps a limit message that has no reset time", func(t *testing.T) {
		long := strings.Repeat("x", excerptLimit+50)
		out := fmt.Appendf(nil, `{"is_error":true,"result":"Usage limit reached %s"}`, long)

		_, err := parseTagsResponse(out, testNow)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "...")
		assert.Less(t, len([]rune(err.Error())), excerptLimit+60)
	})

	t.Run("empty response", func(t *testing.T) {
		_, err := parseTagsResponse([]byte(`{}`), testNow)
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

	if runtime.GOOS == "windows" {
		t.Skip("no sleep binary on windows")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runClaude(ctx, "sleep", "30")
	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}
