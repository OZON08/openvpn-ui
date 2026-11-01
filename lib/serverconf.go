package lib

import (
	"slices"
	"strings"
)

// HandLinesHeader introduces lines of server.conf that no form field renders.
const HandLinesHeader = "# Lines added outside the form, kept by OpenVPN UI"

// repeatableDirectives may appear several times, so a hand-added line can sit next to one of the form.
var repeatableDirectives = []string{"push", "route", "iroute", "setenv"}

// confUnit is one active line of a config, or one inline block such as <ca>...</ca>.
type confUnit struct {
	name string // directive name
	key  string // text without surrounding space, repeated spaces and comments
	text string
}

func parseConf(conf string) []confUnit {
	var units []confUnit
	lines := strings.Split(strings.ReplaceAll(conf, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		fields := strings.Fields(lines[i])
		if len(fields) == 0 || strings.HasPrefix(fields[0], ";") {
			continue
		}
		if tag, isBlock := strings.CutPrefix(fields[0], "<"); isBlock && !strings.HasPrefix(tag, "/") {
			start := i
			for i < len(lines)-1 && strings.TrimSpace(lines[i]) != "</"+tag {
				i++
			}
			text := strings.Join(lines[start:i+1], "\n")
			units = append(units, confUnit{name: fields[0], key: text, text: text})
			continue
		}
		if comment := slices.Index(fields, "#"); comment > 0 {
			fields = fields[:comment]
		}
		units = append(units, confUnit{name: fields[0], key: strings.Join(fields, " "), text: strings.TrimSpace(lines[i])})
	}
	return units
}

// formOwns collects what a rendered config sets, including directives the form has commented out.
func formOwns(conf string) (names, keys map[string]bool) {
	names, keys = map[string]bool{}, map[string]bool{}
	for _, unit := range parseConf(conf) {
		names[strings.TrimPrefix(unit.name, "#")] = true
		keys[unit.key] = true
	}
	return names, keys
}

// HandLines returns the active lines of the current server.conf that saving the form would
// lose: lines that neither the new render nor the render of the former settings hold.
// A directive of the new render belongs to the form, so its value wins over the file.
func HandLines(rendered, previous, current string) string {
	names, keys := formOwns(rendered)
	_, previousKeys := formOwns(previous)
	var kept []string
	seen := map[string]bool{}
	for _, unit := range parseConf(current) {
		if strings.HasPrefix(unit.name, "#") || seen[unit.key] || keys[unit.key] || previousKeys[unit.key] {
			continue
		}
		if names[unit.name] && !slices.Contains(repeatableDirectives, unit.name) {
			continue
		}
		seen[unit.key] = true
		kept = append(kept, unit.text)
	}
	return strings.Join(kept, "\n")
}
