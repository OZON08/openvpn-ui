
Library for interacting with [OpenVPN Management Interface](https://openvpn.net/index.php/open-source/documentation/miscellaneous/79-management-interface.html)

## Password protection

Anyone who can reach the management port can control the server unless it is protected. Set a password file on the server:

```
management 127.0.0.1 2080 /etc/openvpn/management-pw
```

Then pass the password to the client. Load it from your configuration, never from source code:

```go
client := mi.NewClient("tcp", "127.0.0.1:2080")
client.MIPassword = cfg.ManagementPassword
```

The password is sent in cleartext, so keep the interface on localhost. Do not expose it on a public address.

## Unix socket

A Unix socket avoids the network port and the password, because file permissions control access. This is the preferred setup when both sides share a volume:

```
management /etc/openvpn/run/mi.sock unix
```

```go
client := mi.NewClient("unix", "/etc/openvpn/run/mi.sock")
```
