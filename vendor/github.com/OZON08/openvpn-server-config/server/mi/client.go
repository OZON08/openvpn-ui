package mi

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Client is used to connect to OpenVPN Management Interface
type Client struct {
	MINetwork string
	MIAddress string
	// MIPassword is optional. Set it when the server uses a management pw-file.
	MIPassword string `json:"-"`
}

// NewClient initializes Management Interface client structure
func NewClient(network, address string) *Client {
	c := &Client{
		MINetwork: network, //Management Interface Network
		MIAddress: address, //Management Interface Address
	}

	return c
}

// GetPid returns process id of OpenVPN server
func (c *Client) GetPid() (int64, error) {
	str, err := c.Execute("pid")
	if err != nil {
		return -1, err
	}
	return ParsePid(str)
}

// GetVersion returns version of OpenVPN server
func (c *Client) GetVersion() (*Version, error) {
	str, err := c.Execute("version")
	if err != nil {
		return nil, err
	}
	return ParseVersion(str)
}

// GetStatus returns list of connected clients and routing table
func (c *Client) GetStatus() (*Status, error) {
	str, err := c.Execute("status 2")
	if err != nil {
		return nil, err
	}
	return ParseStatus(str)
}

// GetLoadStats returns number of connected clients and total number of network traffic
func (c *Client) GetLoadStats() (*LoadStats, error) {
	str, err := c.Execute("load-stats")
	if err != nil {
		return nil, err
	}
	return ParseStats(str)
}

// KillSession kills OpenVPN connection
func (c *Client) KillSession(cname string) (string, error) {
	str, err := c.Execute("kill " + cname)
	if err != nil {
		return "", err
	}
	return ParseKillSession(str)
}

// Signal sends signal to daemon
func (c *Client) Signal(signal string) error {
	str, err := c.Execute("signal " + signal)
	if err != nil {
		return err
	}
	return ParseSignal(str)
}

// Restarts the OpenVPN server
func (c *Client) RestartServer() error {
	// Send SIGUSR1 signal to restart the server
	err := c.Signal("SIGUSR1")
	if err != nil {
		return err
	}

	return nil
}

const (
	miTimeout        = 10 * time.Second
	miPasswordPrompt = "ENTER PASSWORD:"
)

// login answers the password prompt, which OpenVPN sends without a line ending
func (c *Client) login(conn net.Conn, buf *bufio.Reader) error {
	prompt, err := buf.ReadString(':')
	if err != nil {
		return fmt.Errorf("management login: %w", err)
	}
	if strings.TrimSpace(prompt) != miPasswordPrompt {
		return errors.New("management login: server did not ask for a password")
	}
	if err := writeLine(conn, c.MIPassword); err != nil {
		return fmt.Errorf("management login: %w", err)
	}
	// A wrong password only gets a new prompt without a line ending
	reply, err := buf.ReadString(':')
	if err != nil {
		return fmt.Errorf("management login: %w", err)
	}
	if strings.TrimSpace(reply) != "SUCCESS:" {
		return errors.New("management login: password rejected")
	}
	if _, err := buf.ReadString('\n'); err != nil {
		return fmt.Errorf("management login: %w", err)
	}
	return nil
}

// String keeps the password out of logs and debug output
func (c Client) String() string {
	return c.MINetwork + "://" + c.MIAddress
}

// GoString does the same for the %#v format
func (c Client) GoString() string {
	return c.String()
}

// Execute connects to the OpenVPN server, sends command and reads response
func (c *Client) Execute(cmd string) (string, error) {
	if err := validateLine(cmd); err != nil {
		return "", err
	}
	conn, err := net.DialTimeout(c.MINetwork, c.MIAddress, miTimeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(miTimeout))

	buf := bufio.NewReader(conn)
	if c.MIPassword != "" {
		if err := c.login(conn, buf); err != nil {
			return "", err
		}
	}
	// The prompt has no line ending, so waiting for the welcome line would stall
	if head, _ := buf.Peek(len(miPasswordPrompt)); string(head) == miPasswordPrompt {
		return "", errors.New("management login: server requires a password, set MIPassword")
	}
	if _, err := buf.ReadString('\n'); err != nil {
		return "", fmt.Errorf("management welcome: %w", err)
	}
	err = SendCommand(conn, cmd)
	if err != nil {
		return "", err
	}

	return ReadResponse(buf)
}
