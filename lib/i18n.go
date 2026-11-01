package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const DefaultLanguage = "en"

// SupportedLanguages lists the languages of the UI. English is the source text.
var SupportedLanguages = []string{"de", "en"}

var translations = map[string]map[string]string{}

// LoadTranslations reads one <language>.json file per language, which maps English text to the translation.
func LoadTranslations(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	loaded := map[string]map[string]string{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		texts := map[string]string{}
		if err := json.Unmarshal(raw, &texts); err != nil {
			return err
		}
		loaded[strings.TrimSuffix(filepath.Base(file), ".json")] = texts
	}
	translations = loaded
	return nil
}

// Translate returns the text in the language, or the English text if there is no translation.
func Translate(lang, text string) string {
	if translated, ok := translations[lang][text]; ok {
		return translated
	}
	return text
}

func supportedLanguage(lang string) bool {
	for _, supported := range SupportedLanguages {
		if lang == supported {
			return true
		}
	}
	return false
}

// PreferredLanguage picks the language from the cookie, then from the Accept-Language header.
func PreferredLanguage(cookie, acceptLanguage string) string {
	if supportedLanguage(cookie) {
		return cookie
	}
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		base, _, _ := strings.Cut(strings.ToLower(tag), "-")
		if supportedLanguage(base) {
			return base
		}
	}
	return DefaultLanguage
}
