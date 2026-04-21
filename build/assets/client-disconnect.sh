#!/bin/bash
# OpenVPN client-disconnect hook — posts final session data to openvpn-ui.
#
# Wire it into openvpn server.conf with:
#   script-security 2
#   client-disconnect /etc/openvpn/scripts/client-disconnect.sh
#
# OpenVPN sets these env vars before invoking the script:
#   common_name, trusted_ip, trusted_port, ifconfig_pool_remote_ip,
#   bytes_received, bytes_sent, time_duration, time_unix (connect time_t).
#
# Required env vars for the hook itself (pass via OpenVPN container):
#   OPENVPN_UI_URL         default http://openvpn-ui:8080
#   OPENVPN_UI_HOOK_TOKEN  MUST match MonitoringHookToken in openvpn-ui app.conf
set -eu

UI_URL="${OPENVPN_UI_URL:-http://openvpn-ui:8080}"
TOKEN="${OPENVPN_UI_HOOK_TOKEN:-}"

if [ -z "$TOKEN" ]; then
  echo "[client-disconnect] OPENVPN_UI_HOOK_TOKEN not set — skipping" >&2
  exit 0
fi

payload=$(cat <<EOF
{
  "common_name":  "${common_name:-}",
  "real_ip":      "${trusted_ip:-}",
  "virtual_ip":   "${ifconfig_pool_remote_ip:-}",
  "connected_at": ${time_unix:-0},
  "bytes_in":     ${bytes_received:-0},
  "bytes_out":    ${bytes_sent:-0},
  "duration_s":   ${time_duration:-0}
}
EOF
)

# Fire-and-forget — never block OpenVPN on the webhook.
curl -fsS --max-time 5 \
  -H "Content-Type: application/json" \
  -H "X-Monitor-Token: ${TOKEN}" \
  -X POST \
  --data "${payload}" \
  "${UI_URL}/api/v1/monitor/disconnect" >/dev/null 2>&1 || \
  echo "[client-disconnect] webhook failed (common_name=${common_name:-})" >&2

exit 0
