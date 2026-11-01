# build.sh replaces the placeholder below with the Alpine image for the host architecture
FROM DEFINE-YOUR-ARCH
LABEL maintainer="OZON08 <65833863+OZON08@users.noreply.github.com>"
LABEL version="0.9.7.2"
WORKDIR /opt
EXPOSE 8080/tcp

RUN apk update \
   && apk add --update-cache \
      bash \
      easy-rsa \
      openvpn \
      oath-toolkit-oathtool \
      su-exec \
   && rm -rf /var/cache/apk/* && chmod 755 /usr/share/easy-rsa/* \
   && addgroup -g 61195 openvpn-ui \
   && adduser -D -u 61195 -G openvpn-ui openvpn-ui

ADD assets/start.sh /opt/start.sh
ADD assets/generate_ca_and_server_certs.sh /opt/scripts/generate_ca_and_server_certs.sh
ADD assets/genclient.sh /opt/scripts/genclient.sh
ADD assets/revoke.sh /opt/scripts/revoke.sh
ADD assets/rmcert.sh /opt/scripts/rmcert.sh
ADD assets/remove.sh /opt/scripts/remove.sh
ADD assets/renew.sh /opt/scripts/renew.sh
ADD openvpn-ui.tar.gz /opt/openvpn-ui/
ADD qrencode /opt/scripts/qrencode
RUN rm -f /opt/openvpn-ui/data.db
ADD assets/app.conf /opt/openvpn-ui/conf/app.conf
ADD assets/easyrsa-tools.lib /usr/share/easy-rsa/easyrsa-tools.lib
RUN chown -R openvpn-ui:openvpn-ui /opt/openvpn-ui /opt/scripts && chmod -R g+w /usr/share/easy-rsa

CMD /opt/start.sh