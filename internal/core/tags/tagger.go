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
	"photo/internal/core/proc"
)

//go:embed prompts/stock_photo.txt
var stockPhotoPrompt string

const (
	taggerTimeout = 3 * time.Minute
	taggerModel   = "opus"

	successSubtype   = "success"
	usageLimitMarker = "usage limit reached"
	excerptLimit     = 300
)

// The schema pins the counts the prompt asks for, so the CLI refuses an answer
// with 49 keywords instead of handing it over for the dialog to flag.
var tagsSchema = fmt.Sprintf(`{"type":"object","properties":{`+
	`"title":{"type":"string","maxLength":%d},`+
	`"keywords":{"type":"array","items":{"type":"string"},"minItems":%d,"maxItems":%d,"uniqueItems":true}},`+
	`"required":["title","keywords"],"additionalProperties":false}`,
	model.MaxTitleLength, model.KeywordCount, model.KeywordCount)

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

	// The CLI exits non-zero on a failed run but still writes its full JSON
	// report to stdout, and that report explains the failure far better than
	// the exit status does.
	out, runErr := t.run(ctx, binary, taggerArgs(req)...)
	generated, err := parseTagsResponse(out, t.now())
	switch {
	case err == nil:
		return generated, nil
	case runErr == nil || !errors.Is(err, errNoReport):
		return model.Tags{}, err
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return model.Tags{}, fmt.Errorf("claude timed out after %s: %w", t.timeout, runErr)
	default:
		return model.Tags{}, fmt.Errorf("running claude: %w", runErr)
	}
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

var errNoReport = errors.New("parsing claude response")

func parseTagsResponse(out []byte, now time.Time) (model.Tags, error) {
	var resp claudeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return model.Tags{}, fmt.Errorf("%w: %w (%s)", errNoReport, err, excerpt(out))
	}

	if resp.IsError {
		return model.Tags{}, responseFailure(resp, "claude reported an error", now)
	}

	if resp.StructuredOutput == nil {
		return model.Tags{}, responseFailure(resp, "claude returned no tags", now)
	}

	generated := model.Tags{
		Title:    resp.StructuredOutput.Title,
		Keywords: resp.StructuredOutput.Keywords,
	}
	// An empty answer is a failed run, not a result: taken as one it would blank
	// the fields the dialog was filled with and clear the tags of the sidecar.
	if generated.IsEmpty() {
		return model.Tags{}, responseFailure(resp, "claude returned no tags", now)
	}
	return generated, nil
}

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
	return excerptText(text)
}

// An exhausted subscription arrives as free text, sometimes with the reset time
// appended as "|<unix seconds>".
func usageLimitMessage(text string, now time.Time) (string, bool) {
	if !strings.Contains(strings.ToLower(text), usageLimitMarker) {
		return "", false
	}
	cut := strings.LastIndex(text, "|")
	if cut < 0 {
		return excerptText(text), true
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(text[cut+1:]), 10, 64)
	if err != nil {
		return excerptText(text), true
	}
	return excerptText(strings.TrimSpace(text[:cut])) + ", resets at " + formatReset(time.Unix(seconds, 0), now), true
}

func formatReset(reset, now time.Time) string {
	if reset.Year() == now.Year() && reset.YearDay() == now.YearDay() {
		return reset.Format("15:04")
	}
	return reset.Format("Jan 2, 15:04")
}

func excerpt(data []byte) string {
	return excerptText(string(data))
}

func excerptText(text string) string {
	text = strings.TrimSpace(text)
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
	proc.Hide(cmd)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil && stderr.Len() != 0 {
		return out, fmt.Errorf("%w (%s)", err, excerpt(stderr.Bytes()))
	}
	return out, err
}
