# netpulse mark — design spec

A terminal network toolkit: speed test, LAN speed server, packet loss and MTR, in one static binary with no dependencies.

The netpulse mark is a CognitoBit family mark: a 4×4 grid of sixteen bits. It keeps the family's fixed parts (the grid, the grey "known" bits, and the missing bit at row 3, column 3) and learns a pulse: a signal rising and falling across the grid. The missing bit is the dropped packet.

## Colours

### Light version (on paper, white, or any light background)

| Role | Colour | Hex | Notes |
|---|---|---|---|
| Learning bits | Pulse red | `#C2472F` | The product colour. Use it for the mark, primary buttons and links. |
| Known bits | Warm grey | `#C9C5BC` | Shared by every CognitoBit product mark. Do not tint it per product. |
| Missing bit | none | — | Always transparent. Never fill it. |
| Ink (wordmark, text next to the mark) | Ink | `#141414` | |
| Recommended background | Paper | `#F7F5F0` | Pure white `#FFFFFF` is also fine. |

### Dark version (on ink, black, or any dark background)

| Role | Colour | Hex | Notes |
|---|---|---|---|
| Learning bits | Pulse red, lifted | `#E8785F` | One step lighter so the colour keeps contrast on dark. |
| Known bits | Graphite | `#4A4843` | The dark-mode equivalent of the family grey. |
| Missing bit | none | — | Always transparent. |
| Wordmark, text next to the mark | Paper | `#F7F5F0` | |
| Recommended background | Ink | `#141414` | |

### App icon, filled (coloured tile)

| Role | Colour | Hex / value |
|---|---|---|
| Tile | Pulse red | `#C2472F`, corner radius 23% of the tile (22 px on a 96 px tile) |
| Learning bits | Paper | `#F7F5F0` |
| Known bits | Paper at 28% | `rgba(247, 245, 240, 0.28)`, which flattens to `#D17865` on the tile |
| Mark size | | 54% of the tile, centred (52 px on 96 px) |

### App icon, light (white tile)

| Role | Colour | Hex |
|---|---|---|
| Tile | White | `#FFFFFF`, 1 px border `#DAD6CD`, corner radius 23% |
| Learning bits | Pulse red | `#C2472F` |
| Known bits | Warm grey | `#C9C5BC` |

### Tray and status-bar icons (single colour)

| Variant | Learning bits | Known bits |
|---|---|---|
| White, for dark trays | `#FFFFFF` | `rgba(255, 255, 255, 0.32)` |
| Black, for light trays | `#141414` | `rgba(20, 20, 20, 0.32)` |

The SVG `netpulse-mark-mono.svg` uses `currentColor`, so it takes whichever of these the surrounding CSS sets.

### Favicon

The light version on a transparent background, at 16, 32 and 48 px.

## Geometry (from the CognitoBit logo source)

| Property | Value |
|---|---|
| Grid | 4 × 4 cells |
| Cell size | 15.11 units |
| Cell pitch | 16.67 units (gap 1.56) |
| Corner radius | 2.83 units (18.7% of the cell) |
| Overall box | 65.12 × 65.12 units |

Using percentages, so it scales to any size: gap 2.4% of the box, corner radius 18.7% of the cell.

## Pattern

Row by row, where `c` is a learning bit, `k` a known bit and `m` the missing bit:

```
k c k k
c k c k
k k m c
k k k k
```

Four learning bits trace the pulse. Eleven known bits are the quiet line. The missing bit is the packet that never arrived.

## Type

- Wordmark: Schibsted Grotesk, weight 600, letter-spacing −0.02 em, set as "netpulse" in one word.
- Supporting text: Newsreader (the family's text face).

## Files

- `netpulse-mark.svg`, `netpulse-mark-dark.svg`, `netpulse-mark-mono.svg`: the mark in light, dark and single-colour form.
- `netpulse-app-icon.svg`, `netpulse-app-icon-filled.svg`: the white tile and the coloured tile.
- `netpulse-lockup.svg`: mark plus wordmark (live text, needs Schibsted Grotesk installed).
- `icon-16/24/40/64.png` and `icon-dark-*.png`: the mark on transparent, light and dark.
- `tray-mono-white-*.png`, `tray-mono-black-*.png`: single-colour tray icons at 16, 24 and 32 px.
- `app-icon-256/512.png`, `app-icon-filled-256/512.png`, `netpulse-mark-512.png`.
- `favicon.ico` (16, 32, 48) and `favicon-32.png`.

## Rules of use

- Keep the missing bit empty in every variant, including single-colour and favicon versions.
- Never recolour the known bits per product; grey is what says "CognitoBit".
- Change only the learning bits' shape and colour when creating another product mark.
- Minimum size: 16 px. Below 24 px prefer the single-colour tray versions.
- Clear space: at least one cell pitch (about 25% of the mark) on every side.
