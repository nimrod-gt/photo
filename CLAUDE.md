# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Desktop photo viewer and sorter built with Go + Fyne. Dark theme. Cross-platform (Mac, Windows; Linux optional).

### Core Concept

Photos from cameras are saved as JPEG + RAW (.ARW) pairs. The app displays only JPEGs but all file operations (delete) apply to both the JPEG and its RAW pair.

### UI Layout

- **Left panel**: File browser with directory tree (GoLand-style). Shows only non-RAW files. Sorting by name/time. "Today" button. Color indicators displayed before filenames.
- **Center**: Photo viewer. Favorite and color state is shown only by the top bar buttons and the file list, nothing is drawn over the photo.
- **Top bar**: Action buttons — Favorite, Red, Green, Blue, Tags, Delete.
- **Right-click context menu** on photo: same actions as top bar.

### Features

- **Favorites**: Toggles `xmp:Rating` (5 / 0) in place inside the XMP packet the camera embeds in the JPEG (visible in OS file explorer, Lightroom and on camera). JPEG only; a JPEG without a writable XMP packet is reported and left untouched. Read back as XMP first, EXIF Rating second.
- **Color labels (RGB)**: Toggle Red/Green/Blue labels per photo. A file can have multiple colors simultaneously (e.g., Red + Green). Stored in a `.photo-colors.json` file per folder (maps filenames to color arrays).
- **Color filters**: File browser supports filtering by color — show only files with a specific color label or combination. Filter buttons in the toolbar or left panel.
- **Delete**: Deletes both JPEG and RAW pair. Always asks for confirmation.
- **Settings**: Sort order and direction are remembered between launches. The window always opens maximized.
- **Navigation**: Arrow keys (Right/Down = next, Left/Up = previous) and left mouse click on photo = next. Stays on last/first photo at boundaries.

### Keyboard Shortcuts (layout-independent)

- `F` — toggle Favorite
- `R` — toggle Red
- `G` — toggle Green
- `B` — toggle Blue
- `D` — delete (opens confirmation dialog: `D` = yes, `N`/`Esc` = no)
- `C` — copy photo
- `Y` — copy photo to clipboard
- `Z` — reset zoom
- `+`/`-` — zoom in/out
- `L` — toggle grid view
- `T` — generate stock tags for the current photo
- Arrow keys — navigate photos

### Tech Stack

- Go + Fyne (GUI framework)
- EXIF: `dsoprea/go-exif/v3` + `dsoprea/go-jpeg-image-structure/v2`
- Testing: `stretchr/testify`
- JSON files for color labels
- Linting: `golangci-lint` (go.mod tool dependency)

### Project Layout

GUI code lives under `internal/gui`, everything else under `internal/core`.
Nothing in `internal/core` may import Fyne.

```
main.go
internal/
  core/
    model/      photo, color labels, tags - data types and validation, no I/O
    library/    scanner, navigator, copier, deleter, color service
    imaging/    image loader, thumbnail provider, orientation, EXIF read
    proc/       hidden-window attributes for child processes
    tags/       stock metadata generation via the claude CLI + prompts/
    clipboard/  per-OS clipboard image copy
  gui/
    app/        wiring, actions, dialog state
    ui/         Fyne widgets
```

### Threading

The app is on the `fyne.Do` threading model. It is declared twice: `FyneApp.toml`
(`[Migrations] fyneDo = true`), which is what makes `fyne package` build with the `migrated_fynedo`
tag and compile the checks out, and `declareThreadingMigration` in `internal/gui/app/app.go`, which
holds wherever the binary is launched from. The second one adds the flag to the metadata the app
already has instead of setting it on its own, because `app.SetMetadata` replaces the struct whole.
That declaration turns off Fyne's runtime thread checks, so nothing warns any more when a widget is
touched from the wrong goroutine: every goroutine that reads or changes a widget must wrap that work
in `fyne.Do`. Callbacks handed to `internal/core` run on its worker goroutines and count as such.
`fyne.DoAndWait` deadlocks when called from the main goroutine and is used nowhere.

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
go tool golangci-lint run --fix

# Build
go build ./...

# Cross-compile for Windows (requires: brew install mingw-w64)
# Name, id, icon and version come from FyneApp.toml; the tool bumps its build number afterwards.
CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 fyne package --release --os windows
zip photo-windows-amd64.zip "Photo Viewer.exe"
```

## Code Style

- Follow standard Go conventions (gofmt, golangci-lint)
- Use meaningful variable names (avoid single-letter except for common cases like `i`, `err`)
- Functions should be focused and ideally under 50 lines
- Use table-driven tests where appropriate
- Always handle errors explicitly; use `fmt.Errorf` with `%w` for error wrapping
- Prefer standard library `slices`/`maps` helpers over manual loops where they improve clarity

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
