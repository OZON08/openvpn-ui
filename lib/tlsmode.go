package lib

import "strings"

// ServerTLSLine returns the active tls-auth or tls-crypt line of a server config.
// The config is what OpenVPN really uses, so it decides the mode of client profiles.
func ServerTLSLine(serverConf string) string {
	for _, line := range strings.Split(serverConf, "\n") {
		line = strings.TrimSpace(line)
		if directive, _, _ := strings.Cut(line, " "); directive == "tls-auth" || directive == "tls-crypt" {
			return line
		}
	}
	return ""
}

// ProfileTemplateForServer adapts the client template, which uses tls-crypt, to a server on tls-auth.
func ProfileTemplateForServer(tpl string, serverConf string) string {
	if !strings.HasPrefix(ServerTLSLine(serverConf), "tls-auth") {
		return tpl
	}
	return strings.NewReplacer("<tls-crypt>", "key-direction 1\n<tls-auth>", "</tls-crypt>", "</tls-auth>").Replace(tpl)
}
