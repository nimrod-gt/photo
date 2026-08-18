package service

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
	"strings"
	"time"

	"photo/model"
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

type TagRequest struct {
	Photo      model.Photo
	Notes      string
	ClaudePath string
}

type Tagger struct {
	run     func(ctx context.Context, name string, args ...string) ([]byte, error)
	lookup  func(configured string) (string, error)
	timeout time.Duration
}

func NewTagger() *Tagger {
	return &Tagger{run: runClaude, lookup: LookupClaude, timeout: taggerTimeout}
}

func (t *Tagger) Generate(ctx context.Context, req TagRequest) (model.Tags, error) {
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

	return parseTagsResponse(out)
}

func taggerArgs(req TagRequest) []string {
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

func userMessage(req TagRequest) string {
	message := "Photo: " + req.Photo.ImagePath
	if notes := strings.TrimSpace(req.Notes); len(notes) != 0 {
		message += "\n" + notes
	}
	return message
}

type claudeResponse struct {
	IsError          bool   `json:"is_error"`
	Result           string `json:"result"`
	StructuredOutput *struct {
		Title    string   `json:"title"`
		Keywords []string `json:"keywords"`
	} `json:"structured_output"`
}

func parseTagsResponse(out []byte) (model.Tags, error) {
	var resp claudeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return model.Tags{}, fmt.Errorf("parsing claude response: %w (%s)", err, excerpt(out))
	}

	if resp.IsError {
		return model.Tags{}, fmt.Errorf("claude reported an error: %s", excerpt([]byte(resp.Result)))
	}

	if resp.StructuredOutput == nil {
		return model.Tags{}, fmt.Errorf("claude returned no tags: %s", excerpt([]byte(resp.Result)))
	}

	return model.Tags{
		Title:    resp.StructuredOutput.Title,
		Keywords: resp.StructuredOutput.Keywords,
	}, nil
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
