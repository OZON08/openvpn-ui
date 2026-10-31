package mi

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"
)

// validateLine rejects line breaks, which would smuggle in further commands
func validateLine(line string) error {
	if strings.ContainsAny(line, "\r\n") {
		return errors.New("invalid input: newline characters are not allowed")
	}
	return nil
}

// writeLine sends one protocol line. It never logs, because the line may be a password.
func writeLine(conn net.Conn, line string) error {
	if err := validateLine(line); err != nil {
		return err
	}
	_, err := io.WriteString(conn, line+"\n")
	return err
}

// SendCommand passes command to a given connection (adds logging and EOL character)
func SendCommand(conn net.Conn, cmd string) error {
	if err := writeLine(conn, cmd); err != nil {
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

		// Asynchronous notifications can arrive before the actual reply
		if strings.HasPrefix(line, ">") {
			continue
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
