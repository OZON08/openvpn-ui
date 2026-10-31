// Package render holds the template handling shared by all config packages.
package render

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"text/template"
)

// ownerOnly is the mode of generated files.
const ownerOnly = 0600

// Validator is implemented by configs that check their values before rendering.
type Validator interface {
	Validate() error
}

// CheckStrings runs check on every string field of a config struct.
func CheckStrings(data any, check func(name, value string) error) error {
	v := reflect.ValueOf(data)
	for i := range v.NumField() {
		if f := v.Field(i); f.Kind() == reflect.String {
			if err := check(v.Type().Field(i).Name, f.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

// RequireSingleLine rejects line breaks in the string fields of a config struct,
// because each one would start a new OpenVPN directive. Fields that hold key
// material or raw directives are named in multiline.
func RequireSingleLine(data any, multiline ...string) error {
	return CheckStrings(data, func(name, value string) error {
		if !slices.Contains(multiline, name) && strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("invalid value in %s: line breaks are not allowed", name)
		}
		return nil
	})
}

// RequireKeyMaterial rejects values that could close an inline key block such as
// <cert>. PEM and static key files never contain this character.
func RequireKeyMaterial(values ...string) error {
	for _, value := range values {
		if strings.Contains(value, "<") {
			return errors.New("invalid key material: the character < is not allowed")
		}
	}
	return nil
}

// Text validates data, if it supports that, and injects it into the template.
func Text(tpl string, data any) (string, error) {
	if v, ok := data.(Validator); ok {
		if err := v.Validate(); err != nil {
			return "", err
		}
	}
	t, err := template.New("config").Parse(tpl)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err = t.Execute(buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// SaveToFile renders the template at tplPath to destPath. The result is always
// owner-only, because it can hold private keys. An existing file is updated in
// place, because replacing it would hide the change from containers that bind
// mount this single file. Permissions are tightened before any content changes.
func SaveToFile(tplPath string, data any, destPath string) error {
	tpl, err := os.ReadFile(tplPath)
	if err != nil {
		return err
	}
	txt, err := Text(string(tpl), data)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE, ownerOnly)
	if err != nil {
		return err
	}
	err = f.Chmod(ownerOnly)
	if err == nil {
		err = f.Truncate(0)
	}
	if err == nil {
		_, err = f.WriteString(txt)
	}
	return errors.Join(err, f.Close())
}
