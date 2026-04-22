# Screenshot capture

Playwright script that logs into a running openvpn-ui instance and writes
refreshed PNGs to [../images/](../images/). Filenames match the references
in the top-level [README.md](../../README.md) so re-running the script
just overwrites the existing images.

## Quickstart

```bash
cd docs/screenshots
npm install                         # also runs `playwright install chromium`
cp .env.example .env                # edit UI_URL / UI_USER / UI_PASS
npm run capture
```

Or pass the creds inline without a `.env`:

```bash
UI_URL=http://localhost:8080 UI_USER=admin UI_PASS=secret node capture.mjs
```

The target instance must be reachable from the machine running the script
and the user you log in as must be able to reach every captured page
(Monitor, Logs, Settings, Maintenance — admin account recommended).

## What gets captured

One PNG per view, full-page, at 1400x900 @ 2x DPR:

- Login screen (pre-auth)
- Home, Certificates, Logs, Settings, Profile
- Monitor: Sessions / Users / Retention / InfluxDB tabs
- OpenVPN server & client config, EasyRSA vars, Maintenance

See [capture.mjs](capture.mjs) for the exact view list — add or reorder
entries there if the README layout changes.

## Notes

- Headless Chromium is launched on each run; nothing persists between runs.
- `.env` is in `.gitignore` (don't commit creds); `.env.example` is the
  template you should keep up-to-date if new variables get added.
- If a Monitor tab selector changes, update the `tab:` field on the
  matching view entry — the script just does `page.click('a[href=...]')`.
