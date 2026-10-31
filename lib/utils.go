package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/core/validation"
)

// CreateValidationMap ranslates validation structure to map
// that can be easly presented in template
func CreateValidationMap(valid validation.Validation) map[string]map[string]string {
	v := make(map[string]map[string]string)
	/*
			{
				"email": {
					"Requrired" : "Can not be empty"
				},
				"password" :{

			  }
		  }
	*/
	for _, err := range valid.Errors {
		logs.Notice(err.Key, err.Message)
		k := strings.Split(err.Key, ".")
		var field, errorType string
		if len(k) > 1 {
			field = k[0]
			errorType = k[1]
		} else {
			field = err.Key
			errorType = " "
		}
		logs.Error(field)
		if _, ok := v[field]; !ok {
			v[field] = make(map[string]string)
		}
		v[field][errorType] = err.Message
	}
	return v

}

// Dump any structure as json string
func Dump(obj interface{}) {
	result, _ := json.MarshalIndent(obj, "", "\t")
	logs.Debug(string(result))
}

// WritePrivateFile replaces a file that holds private keys. The owner-only mode is set
// before the content is written, also when the file already exists with a wider mode.
func WritePrivateFile(destPath string, text string) error {
	f, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0600); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err = f.WriteString(text)
	return err
}

// ConfSaveToFile saves the given text to the specified file path, replacing Windows-style line endings with Unix-style line endings.
func ConfSaveToFile(destPath string, text string) error {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	err := os.WriteFile(destPath, []byte(text), 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	return nil
}
