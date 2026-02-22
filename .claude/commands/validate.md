Validate the codebase by running the following commands in sequence. Fix all issues you encounter.

# Instructions

1. **Build the codebase**
   - Run: `go build ./...`
   - Fix all compilation errors before proceeding

2. **Run linter**
   - Run: `golangci-lint run --fix`
   - Review and fix all linting issues reported
   - The `--fix` flag will auto-fix some issues, but you may need to manually fix others

3. **Run tests**
   - Run: `go test -timeout=1m ./...`
   - Fix any failing tests
   - IGNORE tests that fail due to timeout only (these are flaky)
   - Do NOT ignore tests that fail for other reasons (assertion failures, panics, etc.)

# What to report back

- Summary of each step (pass/fail)
- List of issues found and fixed
- Any remaining issues that need manual intervention
- Overall validation status (success/partial/failed)
