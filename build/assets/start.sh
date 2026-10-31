#!/bin/bash
# Exit immediately if a command exits with a non-zero status
set -e

# Directory where OpenVPN configuration files are stored
# Not read from conf/app.conf: the web user can change that file, and this script runs as root
OPENVPN_DIR=${OPENVPN_DIR:-/etc/openvpn}
OPENVPN_DIR=${OPENVPN_DIR%/}
echo "Init. OVPN path: $OPENVPN_DIR"
CONF_DIR=$(grep -E "^OpenVpnPath\s*=" /opt/openvpn-ui/conf/app.conf 2>/dev/null | cut -d= -f2 | tr -d '"' | tr -d '[:space:]')
[ -z "$CONF_DIR" ] || [ "${CONF_DIR%/}" = "$OPENVPN_DIR" ] ||
  echo "WARNING: OpenVpnPath in app.conf differs from OPENVPN_DIR ($OPENVPN_DIR). Set OPENVPN_DIR to the same path."

# Change to the /opt directory
cd /opt/

# If the provisioned file does not exist in the OpenVPN directory, prepare the certificates and create the provisioned file
if [ ! -f $OPENVPN_DIR/.provisioned ]; then
  mkdir -p $OPENVPN_DIR

  # Uncomment line below to generate CA and server certificates (should be done on the side of OpenVPN container or server however)
  #./scripts/generate_ca_and_server_certs.sh

  touch $OPENVPN_DIR/.provisioned
  echo "First OpenVPN UI start."
fi

# Change to the OpenVPN GUI directory
cd /opt/openvpn-ui

# Create the database directory if it does not exist
mkdir -p db

# This script runs as root on paths the web user can write. It skips symlinks, and
# hardlinks, which could point to a file outside of them, such as this script.
UNLINKED=(! -type l \( -type d -o -links 1 \))
find db "${UNLINKED[@]}" -exec chown openvpn-ui:openvpn-ui {} +

# The web process only manages these paths. Anything else, such as fw-rules.sh and
# .env, loses group write, also where earlier versions granted it. The run directory
# is skipped as well, because it restricts access to the management socket.
mkdir -p "$OPENVPN_DIR/config" "$OPENVPN_DIR/clients" "$OPENVPN_DIR/staticclients"
PRUNE=(-path "$OPENVPN_DIR/run")
MANAGED=()
for name in server.conf config clients staticclients pki; do
  PRUNE+=(-o -path "$OPENVPN_DIR/$name")
  [ ! -e "$OPENVPN_DIR/$name" ] || MANAGED+=("$OPENVPN_DIR/$name")
done
find "$OPENVPN_DIR" \( "${PRUNE[@]}" \) -prune -o ! -type l -exec chmod g-w {} + ||
  echo "WARNING: could not remove group write below $OPENVPN_DIR"
# Certificate scripts run as the web user and need to read and write the PKI, including ca.key.
find "${MANAGED[@]}" /usr/share/easy-rsa "${UNLINKED[@]}" -exec chgrp openvpn-ui {} + -exec chmod g+rwX {} + ||
  echo "WARNING: could not grant openvpn-ui access to all managed paths"

# Rendered files are saved with an owner-only mode, which requires owning them.
find "$OPENVPN_DIR/server.conf" "$OPENVPN_DIR/config/client.conf" "$OPENVPN_DIR/config/easy-rsa.vars" \
  "$OPENVPN_DIR/pki/vars" "$OPENVPN_DIR/clients" "${UNLINKED[@]}" -exec chown openvpn-ui {} + 2>/dev/null || true

# Membership in this group grants access to the management socket directory.
# OPENVPN_MI_GID must match the value on the openvpn service.
# A group that already has this GID is reused. A changed GID replaces the old group.
MI_GID=${OPENVPN_MI_GID:-61194}
MI_GROUP=$(awk -F: -v gid="$MI_GID" '$3 == gid { print $1; exit }' /etc/group)
if [ -z "$MI_GROUP" ]; then
  ! grep -q '^openvpn-mi:' /etc/group || delgroup openvpn-mi
  addgroup -g "$MI_GID" openvpn-mi
  MI_GROUP=openvpn-mi
fi
id -nG openvpn-ui | grep -qw "$MI_GROUP" || addgroup openvpn-ui "$MI_GROUP"

# Drop from root to unprivileged openvpn-ui user for the web process
echo "Starting OpenVPN UI!"
exec su-exec openvpn-ui ./openvpn-ui
