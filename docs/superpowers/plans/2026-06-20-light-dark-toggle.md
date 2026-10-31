# Light/Dark Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Re-enable the light/dark toggle in the top navbar so users can switch between themes, with dark mode remaining the default.

**Architecture:** All styles in `v097-custom.css` already use CSS custom properties (`--v097-*`). The only change needed is to make `:root` define light-mode values (making light the CSS-default) and add a `body.dark-mode` block that overrides them with the current dark values. The toggle JS in `base.html` already works correctly — it just needs the HTML uncommented.

**Tech Stack:** CSS custom properties, Go template (Beego), AdminLTE 3.2, Bootstrap 4, localStorage.

## Global Constraints

- Dark mode remains the **default** — the boot script in `base.html` already sets `body.dark-mode` unless `localStorage.getItem('theme') === 'light'`. Do not change this logic.
- No JavaScript changes needed — the toggle handler and boot script at the bottom of `base.html` are complete and correct.
- No Go/controller changes needed — pure frontend (CSS + HTML template).
- The colored stat-box gradients (`.bg-info`, `.bg-success`, warning, pink) stay as-is — they use self-contained colors with white text that work in both modes.
- `custom.css` is not touched — it contains only layout utilities.

---

### Task 1: CSS Variable Light/Dark Split

**Files:**
- Modify: `static/css/v097-custom.css` — change `:root` from dark→light values, add `body.dark-mode` block with dark overrides

**Context:**
The current `:root` block holds dark values. All CSS rules reference these variables. Switching `:root` to light values and moving dark values into `body.dark-mode` is sufficient — every rule already uses the variables.

- [ ] **Step 1: Replace the `:root` block with light-mode defaults**

In `static/css/v097-custom.css`, find the existing `:root { ... }` block (lines 7–25) and replace it with:

```css
:root {
  /* Light mode — default */
  --v097-bg: #f4f6f9;
  --v097-surface: #ffffff;
  --v097-surface-2: #f8f9fa;
  --v097-border: #dee2e6;
  --v097-border-soft: #e9ecef;
  --v097-text: #212529;
  --v097-text-soft: #495057;
  --v097-text-mute: #6c757d;
  --v097-accent: #6366f1;
  --v097-accent-hover: #818cf8;
  --v097-accent-soft: rgba(99,102,241,.15);
  --v097-success: #10b981;
  --v097-warning: #f59e0b;
  --v097-danger: #ef4444;
  --v097-info: #06b6d4;
  --v097-radius: 10px;
  --v097-radius-sm: 6px;
}

body.dark-mode {
  --v097-bg: #0b0d12;
  --v097-surface: #14171f;
  --v097-surface-2: #1a1e28;
  --v097-border: #262b38;
  --v097-border-soft: #1e232d;
  --v097-text: #e5e7eb;
  --v097-text-soft: #9ca3af;
  --v097-text-mute: #6b7280;
}
```

- [ ] **Step 2: Verify no other hardcoded dark colors exist outside colored-box sections**

Scan the rest of `v097-custom.css` for any hardcoded color values that are NOT inside the colored small-box blocks (the `.bg-info`, `.bg-success`, `[style*="ffc107"]`, `[style*="e83e8c"]` rules and the `.small-box > .inner` / `.small-box-footer` rules). The colored-box sections use white text on colored backgrounds — those are intentional and must not be changed.

If any hardcoded dark color appears elsewhere (e.g. `#0b0d12`, `#14171f`, `#1a1e28`, `#262b38`, `#1e232d`, `#e5e7eb`, `#9ca3af`, `#6b7280`), convert it to the appropriate `var(--v097-*)` token.

Expected result: zero hardcoded dark-palette colors outside the colored-box section.

- [ ] **Step 3: Commit**

```bash
git add static/css/v097-custom.css
git commit -m "feat: split CSS variables into light (root) and dark (body.dark-mode) themes"
```

---

### Task 2: Re-enable Toggle in Navbar

**Files:**
- Modify: `views/layout/base.html` — uncomment the toggle `<li>` block (lines 18–25)

**Context:**
The toggle HTML is wrapped in `{{/* ... */}}` (Go template block comment). The JavaScript at the bottom of the file already handles the `checkbox` and `theme-label` element IDs — it just needs the elements to exist.

- [ ] **Step 1: Uncomment the toggle**

In `views/layout/base.html`, find:

```html
          {{/* Light/Dark toggle — hidden for now, forced dark via script below.
          <li class="nav-item">
            <div class="custom-control custom-switch" style="padding-top:14px; margin-right:14px;">
              <input type="checkbox" class="custom-control-input" id="checkbox">
              <label class="custom-control-label" for="checkbox" id="theme-label">Dark</label>
            </div>
          </li>
          */}}
```

Replace with:

```html
          <li class="nav-item">
            <div class="custom-control custom-switch" style="padding-top:14px; margin-right:14px;">
              <input type="checkbox" class="custom-control-input" id="checkbox">
              <label class="custom-control-label" for="checkbox" id="theme-label">Dark</label>
            </div>
          </li>
```

- [ ] **Step 2: Verify the boot script**

Confirm that the `<script>` block at the bottom of `base.html` (after `</html>`) contains exactly this logic — do not modify it, just verify it is present:

```javascript
const checkbox = document.getElementById('checkbox');
const themeLabel = document.getElementById('theme-label');
const body = document.body;
const savedTheme = localStorage.getItem('theme');
if (savedTheme !== 'light') {
  body.classList.add('dark-mode');
  if (checkbox) checkbox.checked = true;
  if (themeLabel) themeLabel.textContent = 'Dark';
} else {
  if (themeLabel) themeLabel.textContent = 'Light';
}
if (checkbox) {
  checkbox.addEventListener('change', function () {
    body.classList.toggle('dark-mode', checkbox.checked);
    if (themeLabel) themeLabel.textContent = checkbox.checked ? 'Dark' : 'Light';
    localStorage.setItem('theme', checkbox.checked ? 'dark' : 'light');
  });
}
```

- [ ] **Step 3: Commit**

```bash
git add views/layout/base.html
git commit -m "feat: re-enable light/dark toggle in top navbar"
```

---

### Task 3: Visual Verification

**Files:** None — manual test only.

- [ ] **Step 1: Start the app or open in browser**

The toggle appears in the top-right navbar area. Default state: toggle **on** (dark mode), label reads "Dark".

- [ ] **Step 2: Switch to light mode**

Click the toggle. Expected:
- Toggle switches off, label reads "Light"
- Body background becomes light grey (`#f4f6f9`)
- Cards/surfaces become white/near-white
- Text becomes dark (`#212529`)
- Sidebar background becomes light
- Navbar becomes light
- Colored stat boxes (blue, green, amber, pink) retain their colored gradients and white text
- Table headers use light grey background
- Forms: inputs have light background, dark text

- [ ] **Step 3: Reload page**

After switching to light, reload the browser. Expected: page loads in light mode (preference persisted in `localStorage`).

- [ ] **Step 4: Switch back to dark mode**

Click toggle again. Expected: immediate return to dark theme (all `--v097-bg`, `--v097-surface` etc. use dark values from `body.dark-mode` override).

- [ ] **Step 5: Reload in dark mode**

Reload again. Expected: page loads in dark mode.

- [ ] **Step 6: Update ROADMAP.md**

Mark the light/dark toggle roadmap item as `[x]` and update its description:

```markdown
- [x] **Light/dark toggle** — The toggle is re-enabled in the top navbar.
      Dark mode is the default; preference is persisted in `localStorage`.
      CSS variables split into `:root` (light defaults) and `body.dark-mode`
      (dark overrides) in `v097-custom.css`.
```

- [ ] **Step 7: Add CHANGELOG entry under `[Unreleased] ### Added`**

```markdown
- **Light/dark toggle re-enabled** — the navbar toggle switches between light
  and dark mode. Dark mode remains the default; the chosen theme is persisted
  in `localStorage`. CSS custom properties are now split into `:root` (light
  defaults) and `body.dark-mode` (dark overrides), so all components respect
  the active theme without JavaScript.
```

- [ ] **Step 8: Commit**

```bash
git add ROADMAP.md CHANGELOG.md
git commit -m "docs: update roadmap and changelog for light/dark toggle"
```
