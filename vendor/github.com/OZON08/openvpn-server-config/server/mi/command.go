package mi

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
)

// SendCommand passes command to a given connection (adds logging and EOL character)
func SendCommand(conn net.Conn, cmd string) error {
	log.Debug("Sending command: " + cmd)
	if _, err := fmt.Fprintf(conn, cmd+"\n"); err != nil {
		return err
	}
	log.Debug("Command ", cmd, " successfuly send")
	return nil
}

const maxResponseLines = 10000

// ReadResponse .
func ReadResponse(reader *bufio.Reader) (string, error) {
	var result = ""

	for i := 0; i < maxResponseLines; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Error(line, err)
			return "", err
		}

		result += line
		if strings.Index(line, "END") == 0 ||
			strings.Index(line, "SUCCESS:") == 0 ||
			strings.Index(line, "ERROR:") == 0 {
			return result, nil
		}
	}
	return "", fmt.Errorf("response exceeded maximum line limit of %d", maxResponseLines)
}
