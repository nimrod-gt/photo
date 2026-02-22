Analyze a Go package and provide a concise summary of its purpose and public interface.

Arguments: `package_path` - relative path to the package (e.g., `internal/service`, `pkg/utils`)

# Instructions

1. **Read the package structure**
   - List all Go files in the package directory (exclude `*_test.go` files initially)
   - Identify the main package files

2. **Analyze public interface**
   - Find all exported (public) types: structs, interfaces, constants, variables
   - Find all exported functions and methods
   - Note any important unexported types that are returned by public functions

3. **Understand the purpose**
   - Read package-level comments if present
   - Analyze the main types and functions to understand what the package does
   - Look for README.md or documentation files in the package directory

4. **Check for examples**
   - Look at test files (`*_test.go`) for example usage patterns
   - Identify common initialization patterns

# What to report back

Provide a structured summary with:

## Package: `package_path`

**Purpose**: 1-2 sentence description of what this package does

**Public Interface**:

### Types
- `TypeName` - brief description
- `InterfaceName` - brief description with key methods

### Constants/Variables
- `ConstantName` - description and value if relevant

### Functions
- `FunctionName(params) returnType` - brief description of what it does
- Group related functions together

### Methods
- `(receiver) MethodName(params) returnType` - brief description
- Only list the most important methods, or summarize if there are many

**Usage Pattern**:
- Brief example or description of typical usage flow
- Initialization requirements (if any)

**Notes**:
- Any important caveats, thread-safety concerns, or special considerations
