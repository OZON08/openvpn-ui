# Auto Theme (OS Sync) Design

## Goal

Replace the binary dark/light checkbox in the navbar with a 3-state segment control: **Light | Auto | Dark**. Auto mode follows the OS `prefers-color-scheme` setting in real time.

## UI

A compact 3-button segment control replaces the `<input type="checkbox">` in the top navbar. Buttons show FontAwesome icons only (no text labels — space is tight):

| Button | Icon | data-theme |
|--------|------|------------|
| Light  | `fas fa-sun` | `light` |
| Auto   | `fas fa-adjust` | `auto` |
| Dark   | `fas fa-moon` | `dark` |

The active button gets the accent background (`--v097-accent`, `#fff` text). Inactive buttons use muted text on a transparent background. The group sits in a pill-shaped container (`var(--v097-surface-2)` background, `var(--v097-radius-sm)` border-radius).

## State Model

`localStorage` key: `'theme'`. Possible values: `'light'` | `'auto'` | `'dark'`.

- **No stored value (null)** → treated as `'auto'` (new default; previously was `'dark'`)
- `'light'` → always light (no OS sync)
- `'auto'` → follows `window.matchMedia('(prefers-color-scheme: dark)')`, updates live when OS setting changes
- `'dark'` → always dark

The `body.dark-mode` class remains the sole theming mechanism — only who sets it changes.

## Architecture

**Two files change:**

### `views/layout/base.html`

1. Replace the `<li>` containing the checkbox/label with a new `<li>` containing three `<button class="theme-btn" data-theme="...">` elements.
2. Replace the `<script>` boot block with a new IIFE that:
   - Reads `localStorage.getItem('theme') || 'auto'` immediately
   - Calls `applyTheme(pref)` synchronously (before paint — prevents flash-of-wrong-theme)
   - Adds `mq.addEventListener('change', ...)` to react to OS changes in auto mode
   - Marks the active button and wires click handlers (DOM is already ready since the script is at the bottom of the document, after `</html>`)

### `static/css/v097-custom.css`

Append styles for `.theme-switcher` (the pill container) and `.theme-btn` (individual buttons). Uses existing CSS variables — no hardcoded colors.

## Backwards Compatibility

- `'light'` in localStorage → still light (unchanged)
- `'dark'` in localStorage → still dark (unchanged)
- `null` (never chose) → now auto instead of dark (intentional improvement)

## No-Flash Guarantee

`applyTheme()` is called at the top of the IIFE, before any event listener registration. Since the script tag is at the end of the document (after `</html>`), `document.body` exists and `classList.toggle` runs synchronously before the browser paints.

## Out of Scope

- No server-side preference storage
- No per-page override
- No animated transition between themes
