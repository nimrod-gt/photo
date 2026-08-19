package tags

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"photo/internal/core/model"
)

//go:embed prompts/stock_photo.txt
var stockPhotoPrompt string

const (
	taggerTimeout = 3 * time.Minute
	taggerModel   = "opus"
	tagsSchema    = `{"type":"object","properties":{"title":{"type":"string"},` +
		`"keywords":{"type":"array","items":{"type":"string"}}},` +
		`"required":["title","keywords"],"additionalProperties":false}`
)

type Request struct {
	Photo      model.Photo
	Notes      string
	ClaudePath string
}

type Tagger struct {
	run     func(ctx context.Context, name string, args ...string) ([]byte, error)
	lookup  func(configured string) (string, error)
	now     func() time.Time
	timeout time.Duration
}

func NewTagger() *Tagger {
	return &Tagger{run: runClaude, lookup: LookupClaude, now: time.Now, timeout: taggerTimeout}
}

func (t *Tagger) Generate(ctx context.Context, req Request) (model.Tags, error) {
	binary, err := t.lookup(req.ClaudePath)
	if err != nil {
		return model.Tags{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	out, err := t.run(ctx, binary, taggerArgs(req)...)
	if err != nil {
		return model.Tags{}, fmt.Errorf("running claude: %w", err)
	}

	return parseTagsResponse(out, t.now())
}

func taggerArgs(req Request) []string {
	return []string{
		"-p", userMessage(req),
		"--system-prompt", stockPhotoPrompt,
		"--allowedTools", "Read",
		"--add-dir", filepath.Dir(req.Photo.ImagePath),
		"--permission-mode", "dontAsk",
		"--safe-mode",
		"--no-session-persistence",
		"--model", taggerModel,
		"--output-format", "json",
		"--json-schema", tagsSchema,
	}
}

func userMessage(req Request) string {
	message := "Photo: " + req.Photo.ImagePath
	if notes := strings.TrimSpace(req.Notes); len(notes) != 0 {
		message += "\n" + notes
	}
	return message
}

type claudeResponse struct {
	IsError          bool   `json:"is_error"`
	Subtype          string `json:"subtype"`
	APIErrorStatus   *int   `json:"api_error_status"`
	Result           string `json:"result"`
	StructuredOutput *struct {
		Title    string   `json:"title"`
		Keywords []string `json:"keywords"`
	} `json:"structured_output"`
}

func parseTagsResponse(out []byte, now time.Time) (model.Tags, error) {
	var resp claudeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return model.Tags{}, fmt.Errorf("parsing claude response: %w (%s)", err, excerpt(out))
	}

	if resp.IsError {
		return model.Tags{}, responseFailure(resp, "claude reported an error", now)
	}

	if resp.StructuredOutput == nil {
		return model.Tags{}, responseFailure(resp, "claude returned no tags", now)
	}

	return model.Tags{
		Title:    resp.StructuredOutput.Title,
		Keywords: resp.StructuredOutput.Keywords,
	}, nil
}

const (
	successSubtype   = "success"
	usageLimitMarker = "usage limit reached"
)

func responseFailure(resp claudeResponse, reason string, now time.Time) error {
	details := []string{reason}
	if len(resp.Subtype) != 0 && resp.Subtype != successSubtype {
		details = append(details, resp.Subtype)
	}
	if resp.APIErrorStatus != nil {
		details = append(details, fmt.Sprintf("HTTP %d", *resp.APIErrorStatus))
	}
	return fmt.Errorf("%s: %s", strings.Join(details, ", "), responseMessage(resp, now))
}

func responseMessage(resp claudeResponse, now time.Time) string {
	text := strings.TrimSpace(resp.Result)
	if limit, ok := usageLimitMessage(text, now); ok {
		return limit
	}
	return excerpt([]byte(text))
}

// An exhausted subscription arrives as free text, sometimes with the reset time
// appended as "|<unix seconds>".
func usageLimitMessage(text string, now time.Time) (string, bool) {
	if !strings.Contains(strings.ToLower(text), usageLimitMarker) {
		return "", false
	}
	head, tail, found := strings.Cut(text, "|")
	head = strings.TrimSpace(head)
	if !found {
		return text, true
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(tail), 10, 64)
	if err != nil {
		return head, true
	}
	return head + ", resets at " + formatReset(time.Unix(seconds, 0), now), true
}

func formatReset(reset, now time.Time) string {
	if reset.Year() == now.Year() && reset.YearDay() == now.YearDay() {
		return reset.Format("15:04")
	}
	return reset.Format("Jan 2, 15:04")
}

const excerptLimit = 300

func excerpt(data []byte) string {
	text := strings.TrimSpace(string(data))
	if len(text) == 0 {
		return "empty response"
	}
	if runes := []rune(text); len(runes) > excerptLimit {
		return string(runes[:excerptLimit]) + "..."
	}
	return text
}

func runClaude(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // binary is resolved by LookupClaude
	cmd.Dir = os.TempDir()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("timed out after %s: %w", taggerTimeout, err)
		}
		if stderr.Len() != 0 {
			return nil, fmt.Errorf("%w (%s)", err, excerpt(stderr.Bytes()))
		}
		return nil, err
	}
	return out, nil
}
