# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Desktop photo viewer and sorter built with Go + Fyne. Dark theme. Cross-platform (Mac, Windows; Linux optional).

### Core Concept

Photos from cameras are saved as JPEG + RAW (.ARW) pairs. The app displays only JPEGs but all file operations (delete) apply to both the JPEG and its RAW pair.

### UI Layout

- **Left panel**: File browser with directory tree (GoLand-style). Shows only non-RAW files. Sorting by name/time. "Today" button. Color indicators displayed before filenames.
- **Center**: Photo viewer. Favorite and color state is shown only by the top bar buttons and the file list, never drawn over the photo. The stock tags are the one exception: title and keyword line as a translucent two-plate overlay in the bottom-left of the image, hidden in grid view and toggled with `I`. The running generations are the other: one plate each in the top-right of the image, shown in grid view as well.
- **Top bar**: Action buttons — Favorite, Red, Green, Blue, Tags, Delete.
- **Right-click context menu** on photo: same actions as top bar.

### Features

- **Favorites**: Toggles `xmp:Rating` (5 / 0) in place inside the XMP packet the camera embeds in the JPEG (visible in OS file explorer, Lightroom and on camera). JPEG only; a JPEG without a writable XMP packet is reported and left untouched. Read back as XMP first, EXIF Rating second.
- **Color labels (RGB)**: Toggle Red/Green/Blue labels per photo. A file can have multiple colors simultaneously (e.g., Red + Green). Stored in a `.photo-colors.json` file per folder (maps filenames to color arrays).
- **Color filters**: File browser supports filtering by color — show only files with a specific color label or combination. Filter buttons in the toolbar or left panel.
- **Stock tags**: `T` opens the Tags dialog; Generate (`Shift+Enter`) runs the claude CLI. While a run is going the row offers Stop (`Esc`), which kills the run and leaves the dialog standing with everything typed into it, and Background (`Ctrl+Enter`), which hides the dialog and lets the run finish; Background over an idle dialog starts a run and sends it away in the same press. A backgrounded run updates the cache and the tag overlay, writes the XMP sidecar of the photo (named after the RAW when there is a pair, after the image itself when there is none, so a JPEG that came without a RAW keeps its tags too; the JPEG file itself is written by Save JPEG alone) and reports itself through a notification. A dialog that closes over a running generation — backgrounded, or clicked away — writes no sidecar of its own: it hands its fields to the run, which writes what it generated, or those fields when it generates nothing, so one writer owns the file. Pressing `T` on a photo whose run is still going re-attaches the dialog to it and puts the handed-over fields back on screen, so the run answers to a dialog again and writes nothing of its own. The sidecar travels only with the RAW: a copy that takes the RAW takes it along and waits for a run still going, while a copy of the image alone — and any copy of a photo that has no RAW — leaves the sidecar where it was. Deleting a photo whose run is still going kills the run and waits for it to let the files go, so no orphan sidecar is left behind. Every run is cancelled when the app exits, and one that was handed a dialog's fields and never landed writes them on the way out. The Concept note is saved with the tags — `photo:Concept` in the sidecar and in the JPEG packet — so reopening the dialog shows what the tags were generated from; a note alone is worth a sidecar, and the EXIF fallback path has no field for it and says so. The Editorial mark and the day it names ride along the same way — `photo:Editorial` and `photo:EditorialDate` in the sidecar and in the JPEG packet — so a photo reopens ticked on the day it was marked for; a mark alone is worth a sidecar too, a generation carries it back with the tags it found, and the EXIF fallback path has no field for it either and says so. Every generation still in flight shows as a plate in the top-right corner of the photo — an icon, the time it has been running and the photo name, oldest on top — and the plate goes when the generation answers or is stopped. Tags travel from one photo to another through the dialog alone: `Alt+C` takes the title and the keywords on screen — the place, the note and the editorial mark stay with the photo they were typed for — and `Alt+V` puts them into another photo's dialog, asking first when that dialog already holds tags. A paste writes no file of its own; the sidecar is written when the dialog closes and the JPEG packet by Save JPEG, the same as anything else typed there.
- **Delete**: Deletes both JPEG and RAW pair. Always asks for confirmation. The XMP sidecar goes once neither of them is left behind — a RAW kept on purpose keeps it, since it also holds the develop settings.
- **Settings**: Sort order and direction, and whether the tag overlay is shown, are remembered between launches. The window always opens maximized.
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
- `I` — toggle the tag overlay
- Arrow keys — navigate photos

The Tags dialog handles its own keys instead of relying on nothing holding focus:

- `Tab`/`Shift+Tab` — move between the fields and the buttons; Cancel and Stop are skipped, since `Esc` is their key
- `Space`/`Enter` — press the focused button, and nothing else: `Enter` in a field never starts a generation, and in Title and Keywords it inserts a newline
- `Shift+Enter` — generate, from anywhere in the dialog
- `Ctrl+Enter` — background the running generation, or start one and background it at once (`Cmd` counts as `Ctrl` here)
- `Alt+C` — copy the title and the keywords on screen
- `Alt+V` — paste them into the dialog of another photo
- `Esc` — stop a running generation and keep the dialog, or close it when nothing is running; it works from every field except the Date one while its calendar popup is open, which swallows the key

The keys are named here the way Windows names them. On macOS the help dialog and the Tags
dialog buttons show `⇧`, `⌃` and `⌥` instead, out of `internal/gui/keyname`, which carries the
names of the modifiers per OS.

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
    filedate/   file creation time, per-OS
    imaging/    image loader, jpeg decoding, thumbnail provider, orientation, EXIF/XMP read, in-place XMP packet writes (favorite, stock tags)
    proc/       hidden-window attributes for child processes
    tags/       stock metadata generation via the claude CLI + prompts/
    claudebin/  claude CLI lookup, per-OS
    clipboard/  per-OS clipboard image copy
  gui/
    app/        wiring, actions, dialog state
    keyname/    names of the Shift/Ctrl/Alt keys, per-OS
    nativewin/  native window maximizing and monitor metrics, per-OS
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
