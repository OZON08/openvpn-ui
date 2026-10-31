#!/bin/bash
# Exit immediately if a command exits with a non-zero status
set -e

# Directory where OpenVPN configuration files are stored
# Not read from conf/app.conf: the web user can change that file, and this script runs as root
OPENVPN_DIR=${OPENVPN_DIR:-/etc/openvpn}
OPENVPN_DIR=${OPENVPN_DIR%/}
echo "Init. OVPN path: $OPENVPN_DIR"

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

# Grant the openvpn-ui group write access to mounted volumes so that the
# non-root process can manage certificates and configs.
chown -R openvpn-ui:openvpn-ui db
# The run directory is skipped: it restricts access to the management socket.
find "$OPENVPN_DIR" -path "$OPENVPN_DIR/run" -prune -o ! -type l -exec chgrp openvpn-ui {} + -exec chmod g+w {} + 2>/dev/null || true
chgrp -R openvpn-ui /usr/share/easy-rsa 2>/dev/null || true
chmod -R g+w /usr/share/easy-rsa 2>/dev/null || true

# Rendered files are saved with an owner-only mode, which requires owning them.
chown -R openvpn-ui "$OPENVPN_DIR/server.conf" "$OPENVPN_DIR/config/client.conf" "$OPENVPN_DIR/config/easy-rsa.vars" \
  "$OPENVPN_DIR/pki/vars" "$OPENVPN_DIR/clients" 2>/dev/null || true

# Membership in this group grants access to the management socket directory.
# OPENVPN_MI_GID must match the value on the openvpn service.
grep -q '^openvpn-mi:' /etc/group || addgroup -g "${OPENVPN_MI_GID:-61194}" openvpn-mi
id -nG openvpn-ui | grep -qw openvpn-mi || addgroup openvpn-ui openvpn-mi

# Drop from root to unprivileged openvpn-ui user for the web process
echo "Starting OpenVPN UI!"
exec su-exec openvpn-ui ./openvpn-ui
