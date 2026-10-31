#!/bin/bash
#VERSION 1.2 by OZON08@github.com aka Mr. Philipp
# Exit immediately if a command exits with a non-zero status
set -e

#Variables
CERT_NAME=$1
CERT_SERIAL=$2
APP_CONF="/opt/openvpn-ui/conf/app.conf"
EASY_RSA="${EASY_RSA:-$(grep -E "^EasyRsaPath\s*=" "$APP_CONF" | cut -d= -f2 | tr -d '"' | tr -d '[:space:]')}"
OPENVPN_DIR="${OPENVPN_DIR:-$(grep -E "^OpenVpnPath\s*=" "$APP_CONF" | cut -d= -f2 | tr -d '"' | tr -d '[:space:]')}"
echo 'EasyRSA path: $EASY_RSA OVPN path: $OPENVPN_DIR'
OVPN_FILE_PATH="$OPENVPN_DIR/clients/$CERT_NAME.ovpn"
QR_CODE_PATH="$OPENVPN_DIR/clients/$CERT_NAME.png" # 2FA QR code
OATH_SECRETS="$OPENVPN_DIR/clients/oath.secrets"   # 2FA secrets file
INDEX="$EASY_RSA/pki/index.txt"

echo "Removing user: $CERT_NAME with Serial: $CERT_SERIAL"

# Only a revoked or expired certificate can be removed
STATUS=$(awk -F'\t' -v serial="$CERT_SERIAL" '$4 == serial { print $1 }' "$INDEX")
if [[ -z $STATUS || $STATUS = "V" ]]; then
    echo "Certificate is valid or unknown. Should not remove: $CERT_NAME with serial: $CERT_SERIAL. Exiting..."
    exit 1
fi

# The record stays in index.txt, because gen-crl only lists the serials it finds there.
# Removing it would make a revoked certificate valid again with the next CRL.
sed -i'.bak' "/${CERT_SERIAL}/ s/$/\/removed=1/" "$INDEX"
rm -f "$OVPN_FILE_PATH"

if ! grep "/CN=$CERT_NAME/" "$INDEX" | grep -qv '/removed=1'; then
    echo "Last certificate of $CERT_NAME removed, cleaning up 2FA data"
    rm -f "$QR_CODE_PATH"
    if [ -f "$OATH_SECRETS" ]; then
        sed -i'.bak' "/${TFA_NAME}/d" "$OATH_SECRETS"
    fi
fi

echo 'Remove done!'
