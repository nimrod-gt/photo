Review all changes made in the current branch compared to the `main` branch for potential issues, Go best practices, and project style compliance.

# Instructions

1. **Get the diff**
   - Run: `git diff main...HEAD` to see all changes from where the branch diverged from main
   - Also run: `git log main..HEAD --oneline` to see commit messages

2. **Review the changes for:**
   - **Potential bugs and errors**:
     - Error handling issues (unchecked errors, improper error wrapping)
     - Race conditions, goroutine leaks
     - Nil pointer dereferences
     - Resource leaks (unclosed connections, files, etc.)
     - Logic errors and edge cases
     - SQL injection or other security issues

   - **Go best practices (go-way)**:
     - Proper use of channels, goroutines, and context
     - Idiomatic Go patterns and conventions
     - Effective use of interfaces and composition
     - Proper error handling with error wrapping
     - Appropriate use of `defer` for cleanup
     - Correct synchronization primitives usage

   - **Project style compliance** (based on CLAUDE.md):
     - No unnecessary comments (avoid "represents...", "returns..." style)
     - Functions under 50 lines
     - Meaningful variable names (no single letters except i, err)
     - Inline error checks (no intermediate error variables)
     - Use `len(v) == 0` instead of `v == ""`
     - Use of testify for assertions in tests

3. **What NOT to analyze**
   - Do NOT provide commit-by-commit analysis
   - Focus on the OVERALL set of changes as a whole

# What to report back

Provide a structured code review with:

- **Critical Issues** (must fix before merge):
  - Potential bugs, security issues, data races
  - Resource leaks, error handling problems
  - Each issue with: file:line reference, description, suggested fix

- **Go Best Practices Violations**:
  - Non-idiomatic code patterns
  - Improper use of Go features
  - Each issue with: file:line reference, description, suggested improvement

- **Project Style Issues**:
  - Violations of project conventions from CLAUDE.md
  - Code that doesn't match established patterns
  - Each issue with: file:line reference, description, how to fix

- **Positive Observations**:
  - Well-written code sections
  - Good use of patterns and practices
  - Improvements over previous code

- **Summary**:
  - Overall code quality assessment
  - Whether the changes are ready for merge or need fixes
  - Priority of issues (critical/high/medium/low)
