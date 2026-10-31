# Auto Theme (OS Sync) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the binary dark/light checkbox in the navbar with a 3-button segment control (Light | Auto | Dark) where Auto follows the OS `prefers-color-scheme` setting in real time.

**Architecture:** Two files change. `v097-custom.css` gets new `.theme-switcher` / `.theme-btn` styles using existing CSS variables. `base.html` replaces the checkbox `<li>` with three icon buttons and replaces the boot script with a new IIFE that reads a 3-state `localStorage` key (`'light'|'auto'|'dark'`, default `'auto'`), applies the theme synchronously to prevent flash, and listens for OS media-query changes.

**Tech Stack:** CSS custom properties, FontAwesome 5.15.3 (free, `fas` class), `window.matchMedia`, `localStorage`, Go template (Beego).

## Global Constraints

- Only `static/css/v097-custom.css` and `views/layout/base.html` are changed
- `body.dark-mode` remains the sole theming mechanism — only who sets it changes
- `localStorage` key is `'theme'`; valid values: `'light'`, `'auto'`, `'dark'`; missing/null treated as `'auto'`
- Existing stored values `'light'` and `'dark'` continue to work unchanged
- FA icons must use the `fas` class (FA5 free, not shadowed by FA4): `fas fa-sun`, `fas fa-adjust`, `fas fa-moon`
- No hardcoded colors in new CSS — use only existing `var(--v097-*)` tokens or `#fff` for active-button text

---

### Task 1: CSS — Segment Control Styles

**Files:**
- Modify: `static/css/v097-custom.css` — append new section at the end of the file

- [ ] **Step 1: Append the segment control styles**

Open `static/css/v097-custom.css` and append the following block at the very end of the file (after the last existing rule):

```css
/* ---------- Theme switcher segment control ---------- */
.theme-switcher {
  display: flex;
  align-items: center;
  gap: 2px;
  background: var(--v097-surface-2);
  border-radius: var(--v097-radius-sm);
  padding: 3px;
  margin: 8px 10px;
}
.theme-btn {
  background: transparent;
  border: none;
  color: var(--v097-text-mute);
  border-radius: 4px;
  padding: 5px 9px;
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  transition: all .15s ease;
}
.theme-btn:hover {
  color: var(--v097-text);
  background: var(--v097-border);
}
.theme-btn.active {
  background: var(--v097-accent) !important;
  color: #fff !important;
}
```

- [ ] **Step 2: Verify no hardcoded dark-palette colors were introduced**

Confirm the new block contains no hardcoded hex values from the dark palette (`#0b0d12`, `#14171f`, `#1a1e28`, `#262b38`, `#1e232d`, `#e5e7eb`, `#9ca3af`, `#6b7280`). Only `#fff` is allowed as a hardcoded color (active-button text).

- [ ] **Step 3: Commit**

```bash
git add static/css/v097-custom.css
git commit -m "feat: add segment control styles for 3-state theme switcher"
```

---

### Task 2: HTML + JS — Segment Control and Boot Script

**Files:**
- Modify: `views/layout/base.html`
  - Replace the checkbox `<li>` block with three icon buttons
  - Replace the entire `<script>` boot block with new 3-state IIFE

**Context from Task 1:** `.theme-switcher` and `.theme-btn` CSS classes are now available.

- [ ] **Step 1: Replace the checkbox `<li>` with the segment control**

In `views/layout/base.html`, find this block (currently the toggle in the navbar):

```html
          <li class="nav-item">
            <div class="custom-control custom-switch" style="padding-top:14px; margin-right:14px;">
              <input type="checkbox" class="custom-control-input" id="checkbox">
              <label class="custom-control-label" for="checkbox" id="theme-label">Dark</label>
            </div>
          </li>
```

Replace it with:

```html
          <li class="nav-item">
            <div class="theme-switcher">
              <button class="theme-btn" data-theme="light" title="Light"><i class="fas fa-sun"></i></button>
              <button class="theme-btn" data-theme="auto"  title="Auto"><i class="fas fa-adjust"></i></button>
              <button class="theme-btn" data-theme="dark"  title="Dark"><i class="fas fa-moon"></i></button>
            </div>
          </li>
```

- [ ] **Step 2: Replace the `<script>` boot block**

In `views/layout/base.html`, find the entire `<script>...</script>` block at the bottom of the file (after `</html>`):

```html
<script>
  // Theme toggle — persisted in localStorage
  const checkbox = document.getElementById('checkbox');
  const themeLabel = document.getElementById('theme-label');
  const body = document.body;
  const savedTheme = localStorage.getItem('theme');
  if (savedTheme !== 'light') {
    // v0.9.7 is dark by default
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
</script>
```

Replace it with:

```html
<script>
  (function () {
    var mq = window.matchMedia('(prefers-color-scheme: dark)');

    function applyTheme(pref) {
      var isDark = pref === 'dark' || (pref === 'auto' && mq.matches);
      document.body.classList.toggle('dark-mode', isDark);
    }

    var saved = localStorage.getItem('theme') || 'auto';
    applyTheme(saved);

    mq.addEventListener('change', function () {
      if ((localStorage.getItem('theme') || 'auto') === 'auto') {
        applyTheme('auto');
      }
    });

    document.querySelectorAll('.theme-btn').forEach(function (btn) {
      if (btn.dataset.theme === saved) btn.classList.add('active');
      btn.addEventListener('click', function () {
        var pref = btn.dataset.theme;
        localStorage.setItem('theme', pref);
        document.querySelectorAll('.theme-btn').forEach(function (b) {
          b.classList.remove('active');
        });
        btn.classList.add('active');
        applyTheme(pref);
      });
    });
  })();
</script>
```

- [ ] **Step 3: Verify `id="checkbox"` and `id="theme-label"` are gone**

Confirm neither `id="checkbox"` nor `id="theme-label"` appear anywhere in `views/layout/base.html` after the changes — they belonged to the old checkbox UI and must not remain.

- [ ] **Step 4: Commit**

```bash
git add views/layout/base.html
git commit -m "feat: replace dark/light checkbox with 3-state theme segment control (light/auto/dark)"
```

---

### Task 3: Docs Update

**Files:**
- Modify: `ROADMAP.md` — no new roadmap item needed; auto-theme is an enhancement of the already-completed toggle item. Add a note to the existing `[x]` entry.
- Modify: `CHANGELOG.md` — add entry under `[Unreleased] ### Changed`

- [ ] **Step 1: Update ROADMAP entry**

In `ROADMAP.md`, find the existing completed toggle item:

```markdown
- [x] **Light/dark toggle** — The toggle is re-enabled in the top navbar.
      Dark mode is the default; preference is persisted in `localStorage`.
      CSS variables split into `:root` (light defaults) and `body.dark-mode`
      (dark overrides) in `v097-custom.css`.
```

Replace with:

```markdown
- [x] **Light/dark toggle** — Three-state theme switcher in the top navbar:
      Light, Auto (follows OS `prefers-color-scheme`, default), and Dark.
      Preference is persisted in `localStorage`. CSS variables split into
      `:root` (light defaults) and `body.dark-mode` (dark overrides) in
      `v097-custom.css`; OS changes in Auto mode are applied live.
```

- [ ] **Step 2: Add CHANGELOG entry**

In `CHANGELOG.md`, find the `[Unreleased]` section and add a `### Changed` block after the existing `### Added` / `### Fixed` entries:

```markdown
### Changed

- **Theme switcher upgraded to 3-state segment control** — the binary
  dark/light checkbox is replaced by a compact icon button group
  (☀ Light | ⊙ Auto | ☾ Dark). Auto mode follows the OS
  `prefers-color-scheme` media query and updates live when the OS
  setting changes. Auto is the new default for users with no stored
  preference; existing `'light'` and `'dark'` stored values are unchanged.
```

- [ ] **Step 3: Commit**

```bash
git add ROADMAP.md CHANGELOG.md
git commit -m "docs: update roadmap and changelog for 3-state theme switcher"
```
