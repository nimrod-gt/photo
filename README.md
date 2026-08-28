# photo

Desktop photo viewer and sorter for camera photos, built with Go and [Fyne](https://fyne.io). Dark theme, cross-platform (macOS, Windows).

Cameras save photos as JPEG + RAW (`.ARW`) pairs. The app displays JPEGs, but file operations (delete, copy) apply to both the JPEG and its RAW pair.

## Features

- **File browser** with directory tree, sorting by name/time, color indicators
- **Favorites** toggled in place in the JPEG's embedded XMP (`xmp:Rating`), the way the camera does it — the file keeps its size and layout, so a Sony body keeps showing it; read from XMP, then from the EXIF Rating field
- **Color labels** (Red/Green/Blue) per photo, stored in a `.photo-colors.json` file per folder; a photo can carry several colors at once
- **Color filters** — show only photos with selected labels, with bulk delete/copy of the filtered set
- **Delete** removes both JPEG and RAW pair (with confirmation)
- **Copy** single photo or all filtered photos, with or without RAW
- **Grid view**, zoom/pan, EXIF-orientation-aware rendering

## Metadata written to files

Everything here is meant to survive the trip to a microstock agency through Lightroom and, sometimes,
Photoshop. What the app writes, and where:

| Field | Carrier | Notes |
| --- | --- | --- |
| Favorite | `xmp:Rating` (5/0) | Patched in place inside the JPEG's embedded XMP packet, so the file keeps its size and layout and the camera keeps showing it. Read back from XMP first, EXIF `Rating` second |
| Title | `dc:title`, `dc:description` | Both, because stock sites disagree on which one carries the caption |
| Keywords | `dc:subject` | 50 flat keywords |
| Location | `Iptc4xmpCore:Location` | The free text typed in the Tags dialog |
| Location, split | `photoshop:City`, `photoshop:State`, `photoshop:Country` | Only when the generator manages to split the free text; a level it is unsure of stays empty |

Title, keywords and location go into the `.xmp` sidecar beside the RAW pair and into the JPEG's XMP
packet. Which of the two is written on its own and which waits for the Save button is set in the
settings (`I`). Where the packet has no room the title and keywords fall back to the EXIF
(`ImageDescription`, `XPTitle`, `XPKeywords`) and the whole file is rewritten, which is reported.
There is no EXIF tag for a place, so on that path the location reaches the sidecar only, and the app
says so.

## Keyboard shortcuts

| Key | Action |
| --- | --- |
| `F` | Toggle favorite |
| `R` / `G` / `B` | Toggle Red / Green / Blue label |
| `D` | Delete (confirm with `D`, cancel with `N`/`Esc`) |
| `C` | Copy photo |
| `Y` | Copy photo to clipboard |
| `Z` | Reset zoom |
| `+` / `-` | Zoom in / out |
| `L` | Toggle grid view |
| `T` | Generate stock tags |
| `I` | Open the settings |
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
