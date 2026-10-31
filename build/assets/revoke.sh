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
INDEX=$EASY_RSA/pki/index.txt
OVPN_FILE_PATH="$OPENVPN_DIR/clients/$CERT_NAME.ovpn"
ISSUED="$EASY_RSA/pki/issued/$CERT_NAME.crt"
RENEWED="$EASY_RSA/pki/renewed/issued/$CERT_NAME.crt"

export EASYRSA_BATCH=1 # see https://superuser.com/questions/1331293/easy-rsa-v3-execute-build-ca-and-gen-req-silently

serial_of() {
    if [[ -f $1 ]]; then openssl x509 -noout -serial -in "$1" | cut -d= -f2; fi
}

# Revokes one certificate with the given easyrsa command. The record stays in index.txt,
# because gen-crl only lists the serials it finds there.
revoke_certificate() {
    local command=$1 serial=$2
    echo "Revoking certificate with serial $serial using: easyrsa $command"
    # easyrsa compares the subject, so the /name suffix has to go first
    sed -i'.bak' "/$serial/s/\/name=$CERT_NAME.*//" "$INDEX"
    ./easyrsa "$command" "$CERT_NAME"
    # Note: CERT_IP is intentionally omitted, the UI does not pass it for revoke operations.
    sed -i'.bak' "/$serial/ s/$/\/name=${CERT_NAME}\/2FAName=${TFA_NAME}/" "$INDEX"
}

ISSUED_SERIAL=$(serial_of "$ISSUED")
RENEWED_SERIAL=$(serial_of "$RENEWED")
cd "$EASY_RSA"

if [[ -n $RENEWED_SERIAL && $CERT_SERIAL = "$RENEWED_SERIAL" ]]; then
    # The old certificate of a renewal: the new one stays valid
    revoke_certificate revoke-renewed "$RENEWED_SERIAL"
elif [[ -n $ISSUED_SERIAL && $CERT_SERIAL = "$ISSUED_SERIAL" ]]; then
    revoke_certificate revoke "$ISSUED_SERIAL"
    # The old certificate would otherwise keep working after the current one is revoked
    if [[ -n $RENEWED_SERIAL ]]; then
        revoke_certificate revoke-renewed "$RENEWED_SERIAL"
    fi
else
    echo "No valid certificate of $CERT_NAME has serial $CERT_SERIAL. Exiting..."
    exit 1
fi

# The UI renders a new profile on download. A file left here could hold a revoked certificate.
rm -f "$OVPN_FILE_PATH"

echo 'Create new certificate revocation list (CRL)...'
./easyrsa gen-crl
chmod +r "$EASY_RSA/pki/crl.pem"

echo -e 'Done!\nIf you want to disconnect the user please restart the service using docker-compose restart openvpn.'
