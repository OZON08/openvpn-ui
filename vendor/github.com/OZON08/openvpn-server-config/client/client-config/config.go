package clientconfig

import (
	"errors"

	"github.com/OZON08/openvpn-server-config/internal/render"
)

var defaultConfig = Config{
	FuncMode:          0, // 0 = standard authentication (cert, cert + password), 1 = 2FA authentication (cert + OTP)
	Device:            "tun",
	Proto:             "udp",
	ServerAddress:     "127.0.0.1",
	Port:              1194,
	OpenVpnServerPort: "1194",
	ResolveRetry:      "resolv-retry infinite",
	OVClientUser:      "nobody",
	OVClientGroup:     "nogroup",
	PersistTun:        "persist-tun",
	PersistKey:        "persist-key",
	RemoteCertTLS:     "remote-cert-tls server",
	Cipher:            "AES-256-GCM",
	RedirectGateway:   "redirect-gateway def1",
	Auth:              "SHA512",
	AuthNoCache:       "auth-nocache",
	TlsClient:         "tls-client",
	Verbose:           "3",
	Ca:                "ca.crt",
	AuthUserPass:      "",                 // "auth-user-pass" when 2fa
	TFAIssuer:         "MFA%20OpenVPN-UI", // 2FA issuer
	CustomConfOne:     "#Custom Option One",
	CustomConfTwo:     "#Custom Option Two",
	CustomConfThree:   "#Custom Option Three",
}

// Config model
type Config struct {
	FuncMode          int
	Device            string
	ServerAddress     string
	Port              int
	ResolveRetry      string
	OVClientUser      string
	OVClientGroup     string
	PersistTun        string
	PersistKey        string
	RemoteCertTLS     string
	OpenVpnServerPort string
	Proto             string

	Ca   string
	Cert string
	Key  string
	Ta   string

	Cipher          string
	RedirectGateway string
	Auth            string
	AuthNoCache     string
	TlsClient       string
	Verbose         string
	AuthUserPass    string
	TFAIssuer       string

	CustomConfOne   string
	CustomConfTwo   string
	CustomConfThree string
}

// New returns config object with default values
func New() Config {
	return defaultConfig
}

// GetText injects config values into template
func GetText(tpl string, c Config) (string, error) {
	return render.Text(tpl, c)
}

// SaveToFile reads template and writes result to destination file
func SaveToFile(tplPath string, c Config, destPath string) error {
	return render.SaveToFile(tplPath, c, destPath)
}

// Validate rejects line breaks outside the key material and raw directive fields
// and values that could close an inline key block
func (c Config) Validate() error {
	return errors.Join(
		render.RequireSingleLine(c, "Ca", "Cert", "Key", "Ta", "CustomConfOne", "CustomConfTwo", "CustomConfThree"),
		render.RequireKeyMaterial(c.Ca, c.Cert, c.Key, c.Ta),
	)
}
