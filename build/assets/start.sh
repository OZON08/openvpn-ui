#!/bin/bash
# Exit immediately if a command exits with a non-zero status
set -e

# Directory where OpenVPN configuration files are stored
OPENVPN_DIR=$(grep -E "^OpenVpnPath\s*=" openvpn-ui/conf/app.conf | cut -d= -f2 | tr -d '"' | tr -d '[:space:]')
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
chgrp -R openvpn-ui "$OPENVPN_DIR" 2>/dev/null || true
chmod -R g+w "$OPENVPN_DIR" 2>/dev/null || true
chgrp -R openvpn-ui /usr/share/easy-rsa 2>/dev/null || true
chmod -R g+w /usr/share/easy-rsa 2>/dev/null || true

# Drop from root to unprivileged openvpn-ui user for the web process
echo "Starting OpenVPN UI!"
exec su-exec openvpn-ui ./openvpn-ui
