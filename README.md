# photo

Desktop photo viewer and sorter for camera photos, built with Go and [Fyne](https://fyne.io). Dark theme, cross-platform (macOS, Windows).

Cameras save photos as JPEG + RAW (`.ARW`) pairs. The app displays JPEGs, but file operations (delete, copy) apply to both the JPEG and its RAW pair.

## Features

- **File browser** with directory tree, sorting by name/time, color indicators
- **Favorites** read from the EXIF Rating field
- **Color labels** (Red/Green/Blue) per photo, stored in a `.photo-colors.json` file per folder; a photo can carry several colors at once
- **Color filters** — show only photos with selected labels, with bulk delete/copy of the filtered set
- **Delete** removes both JPEG and RAW pair (with confirmation)
- **Copy** single photo or all filtered photos, with or without RAW
- **Grid view**, zoom/pan, EXIF-orientation-aware rendering

## Keyboard shortcuts

| Key | Action |
| --- | --- |
| `R` / `G` / `B` | Toggle Red / Green / Blue label |
| `D` | Delete (confirm with `D`, cancel with `N`/`Esc`) |
| `C` | Copy photo |
| `Y` | Copy photo to clipboard |
| `Z` | Reset zoom |
| `+` / `-` | Zoom in / out |
| `L` | Toggle grid view |
| Arrow keys | Navigate photos |

## Build

```bash
go build ./...
```

Cross-compile for Windows (requires `brew install mingw-w64`):

```bash
CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
  fyne package --release --icon Icon.png --app-id com.photo.viewer --os windows
```

## Development

```bash
go test -timeout=1m -shuffle=on ./...
go tool golangci-lint run --fix
```

## License

MIT — see [LICENSE](LICENSE).
