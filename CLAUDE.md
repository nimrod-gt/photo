# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go project. <!-- TODO: Add project description -->

## Commands

### Building and Testing

```bash
# Run tests
go test -v -timeout=1m -shuffle=on ./...

# Run tests for a specific package
go test -v ./path/to/package/...

# Run a single test
go test -v -run TestSpecificFunction ./path/to/package

# Run linting
golangci-lint run --fix

# Build
go build ./...
```

## Code Style

- Follow standard Go conventions (gofmt, golangci-lint)
- Use meaningful variable names (avoid single-letter except for common cases like `i`, `err`)
- Functions should be focused and ideally under 50 lines
- Use table-driven tests where appropriate
- Always handle errors explicitly; use `pkg/errors` for error wrapping
- Use `shopspring/decimal` for financial calculations (never float64)
- Prefer utility functions over manual loops:
  - Use `slices.Map`, `slices.Filter`, `slices.Reduce` from `github.com/robdavid/genutil-go/slices`
  - Use functional programming utilities from helper packages where appropriate

### Code Comments Policy

- **DO NOT** add comments to code unless they explain complex business logic or non-obvious behavior
- **NO** comments that simply restate what the code does (e.g., "// User represents a user")
- **NO** "represents...", "returns...", "contains..." style comments for types, structs, or functions
- The code should be self-documenting through clear naming
- Only add comments when the "why" is not obvious from the code itself

### Git Workflow

- The main branch is `main`
- Never commit any changes without direct order

### Value Checks

- For strings always use `len(v) == 0` instead of `v == ""`

### Error Handling Policy

- **Always use inline error checks** when the error needs to be checked but the result is discarded
- **NEVER** use intermediate error variables like `markErr`, `updateErr`, etc. when checking nested error calls
- **Example of CORRECT pattern:**
  ```go
  if _, err := m.manager.DoSomething(ctx, id); err != nil {
      log.Errorf(ctx, "Failed to do something: %v", err)
  }
  ```
- **Example of INCORRECT pattern:**
  ```go
  _, markErr := m.manager.DoSomething(ctx, id)
  if markErr != nil {
      log.Errorf(ctx, "Failed to do something: %v", markErr)
  }
  ```
