// Package monitor parses openvpn-status.log and feeds long-term traffic
// metrics into both SQLite (local, with retention) and InfluxDB v3 (optional
// time-series backend).
package monitor

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// StatusClient is one row from the "CLIENT LIST" section of openvpn-status.log.
// Byte counters are cumulative per session, not per sample.
type StatusClient struct {
	CommonName    string
	RealIP        string
	RealPort      string
	BytesReceived int64
	BytesSent     int64
	ConnectedAt   time.Time
	// VirtualIP is populated from the ROUTING TABLE section, keyed by CommonName.
	VirtualIP string
}

// ParseStatusLog reads an OpenVPN status file (v2 or v3 format) and returns
// the set of currently connected clients. Unknown lines are skipped; the
// parser is tolerant of timestamp locale differences.
func ParseStatusLog(path string) ([]StatusClient, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	clients := map[string]*StatusClient{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		switch {
		case line == "OpenVPN CLIENT LIST":
			section = "client-v2"
			continue
		case line == "ROUTING TABLE":
			section = "routing-v2"
			continue
		case line == "GLOBAL STATS":
			section = "stats"
			continue
		case line == "END":
			section = "end"
			continue
		case strings.HasPrefix(line, "TITLE,") || strings.HasPrefix(line, "TIME,"):
			// v3 preamble lines — ignore
			continue
		case strings.HasPrefix(line, "HEADER,CLIENT_LIST"):
			section = "client-v3"
			continue
		case strings.HasPrefix(line, "HEADER,ROUTING_TABLE"):
			section = "routing-v3"
			continue
		case strings.HasPrefix(line, "HEADER,"):
			section = "header-other"
			continue
		case strings.HasPrefix(line, "CLIENT_LIST,"):
			if c := parseV3ClientLine(line); c != nil {
				clients[c.CommonName] = c
			}
			continue
		case strings.HasPrefix(line, "ROUTING_TABLE,"):
			vip, cn := parseV3RoutingLine(line)
			if cn != "" {
				if c, ok := clients[cn]; ok && c.VirtualIP == "" {
					c.VirtualIP = vip
				}
			}
			continue
		}

		switch section {
		case "client-v2":
			// Skip the column header row.
			if strings.HasPrefix(line, "Common Name,") {
				continue
			}
			if c := parseV2ClientLine(line); c != nil {
				clients[c.CommonName] = c
			}
		case "routing-v2":
			if strings.HasPrefix(line, "Virtual Address,") {
				continue
			}
			vip, cn := parseV2RoutingLine(line)
			if cn != "" {
				if c, ok := clients[cn]; ok && c.VirtualIP == "" {
					c.VirtualIP = vip
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	out := make([]StatusClient, 0, len(clients))
	for _, c := range clients {
		out = append(out, *c)
	}
	return out, nil
}

// parseV2ClientLine parses:
//   Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since[,Connected Since (time_t)[,Username[,Client ID[,Peer ID[,Data Channel Cipher]]]]]
func parseV2ClientLine(line string) *StatusClient {
	fields := strings.Split(line, ",")
	if len(fields) < 5 {
		return nil
	}
	c := &StatusClient{CommonName: fields[0]}
	c.RealIP, c.RealPort = splitHostPort(fields[1])
	c.BytesReceived, _ = strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
	c.BytesSent, _ = strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64)
	// Prefer the time_t column when present — it is locale-free.
	if len(fields) >= 6 {
		if ts, err := strconv.ParseInt(strings.TrimSpace(fields[5]), 10, 64); err == nil && ts > 0 {
			c.ConnectedAt = time.Unix(ts, 0).UTC()
		}
	}
	if c.ConnectedAt.IsZero() {
		c.ConnectedAt = parseConnectedSince(fields[4])
	}
	return c
}

// parseV2RoutingLine parses:
//   Virtual Address,Common Name,Real Address,Last Ref[,Last Ref (time_t)]
func parseV2RoutingLine(line string) (virtualIP, commonName string) {
	fields := strings.Split(line, ",")
	if len(fields) < 3 {
		return "", ""
	}
	return fields[0], fields[1]
}

// parseV3ClientLine parses:
//   CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6 Address,
//     Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t),
//     Username,Client ID,Peer ID,Data Channel Cipher
func parseV3ClientLine(line string) *StatusClient {
	fields := strings.Split(line, ",")
	if len(fields) < 9 {
		return nil
	}
	c := &StatusClient{CommonName: fields[1]}
	c.RealIP, c.RealPort = splitHostPort(fields[2])
	c.VirtualIP = fields[3]
	c.BytesReceived, _ = strconv.ParseInt(strings.TrimSpace(fields[5]), 10, 64)
	c.BytesSent, _ = strconv.ParseInt(strings.TrimSpace(fields[6]), 10, 64)
	if ts, err := strconv.ParseInt(strings.TrimSpace(fields[8]), 10, 64); err == nil && ts > 0 {
		c.ConnectedAt = time.Unix(ts, 0).UTC()
	} else {
		c.ConnectedAt = parseConnectedSince(fields[7])
	}
	return c
}

// parseV3RoutingLine parses:
//   ROUTING_TABLE,Virtual Address,Common Name,Real Address,Last Ref,Last Ref (time_t)
func parseV3RoutingLine(line string) (virtualIP, commonName string) {
	fields := strings.Split(line, ",")
	if len(fields) < 4 {
		return "", ""
	}
	return fields[1], fields[2]
}

// parseConnectedSince accepts OpenVPN's textual timestamp, e.g.
//   "Thu May 13 15:00:00 2021"
// Returns zero time if it cannot be parsed.
func parseConnectedSince(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.ANSIC,                      // "Mon Jan _2 15:04:05 2006"
		"Mon Jan  2 15:04:05 2006",       // double-space variant
		"Mon Jan 2 15:04:05 2006",
		time.RFC1123,
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// splitHostPort handles both "1.2.3.4:54321" and "[::1]:54321", as well as
// bare hostnames. It is tolerant of missing ports.
func splitHostPort(s string) (host, port string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	return s, ""
}

// FormatAddr is a small helper for log lines and UI output.
func FormatAddr(host, port string) string {
	if port == "" {
		return host
	}
	return fmt.Sprintf("%s:%s", host, port)
}
