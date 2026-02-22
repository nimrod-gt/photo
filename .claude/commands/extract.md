Extract and analyze Go package paths and structure for subsequent work.

Arguments: `[package_paths...]` - optional space-separated relative paths (e.g., `internal/service pkg/utils`). If no paths provided, analyze key project packages.

# Instructions

1. **Identify target packages**
   - If paths provided: use those specific paths
   - If no paths: analyze key areas based on project structure
   - Validate that paths exist in the project

2. **For each package path, extract:**
   - **Package structure**:
     - List all `.go` files (exclude `*_test.go` initially)
     - Count of source files and test files
     - Subdirectories/subpackages if any

   - **Import dependencies**:
     - Run `go list -f '{{.ImportPath}}: {{join .Imports "\n"}}' ./path` to get imports
     - Categorize imports: stdlib, internal (project), external (third-party)
     - Note key internal dependencies (other packages)

   - **Public API surface**:
     - List exported types (structs, interfaces)
     - List exported functions
     - Count of public vs private identifiers

   - **File paths for tool usage**:
     - Full absolute paths to main source files
     - Path patterns for glob operations (e.g., `internal/service/**/*.go`)
     - Key files to read first (e.g., `client.go`, `service.go`, `handler.go`)

3. **Generate work context**
   - Suggest which files to read first for understanding the package
   - Provide glob patterns for searching within packages
   - Identify configuration files related to packages

# What to report back

Provide a structured analysis:

## Package Analysis Results

### Analyzed Packages
- `package/path1` - brief description (1 line)
- `package/path2` - brief description (1 line)

---

### Detailed Breakdown

For each package:

#### `package/path/here`

**Location**: `/absolute/path/to/package`

**Structure**:
- Source files: N files
- Test files: N files
- Subpackages: list if any
- Key files: `client.go`, `service.go`, etc.

**Public API**:
- Types: `Client`, `Config`, `Handler` (N total)
- Functions: `NewClient()`, `Start()`, ... (N total)
- Interfaces: `Service`, `Repository` (N total)

**File Paths for Tools**:
```
# Read key files
/absolute/path/to/package/client.go
/absolute/path/to/package/service.go

# Glob pattern for all sources
package/path/**/*.go

# Grep within package
-path package/path/
```

---

### Recommended Work Approach

1. **Start by reading these files** (ordered by importance):
   - `/path/to/file1.go` - main entry point
   - `/path/to/file2.go` - core logic

2. **Use these patterns for searching**:
   - Glob: `internal/service/**/*.go` for all Go files
   - Grep: `-path internal/service/` to search within package

3. **Testing**:
   - Test command: `go test -v ./package/path/...`

### Summary

Brief summary of what was analyzed and key findings. Note any interesting patterns, potential issues, or areas that need attention.
