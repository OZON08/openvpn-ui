package lib

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTranslateFallsBackToEnglish(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "de.json"), []byte(`{"Save":"Speichern"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadTranslations(dir); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ lang, text, want string }{
		{"de", "Save", "Speichern"},
		{"de", "Unknown text", "Unknown text"},
		{"en", "Save", "Save"},
		{"fr", "Save", "Save"},
		{"", "Save", "Save"},
	} {
		if got := Translate(tc.lang, tc.text); got != tc.want {
			t.Errorf("Translate(%q, %q) = %q, want %q", tc.lang, tc.text, got, tc.want)
		}
	}
}

func TestLoadTranslationsRejectsBrokenFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "de.json"), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadTranslations(dir); err == nil {
		t.Error("expected an error for a broken file")
	}
}

func TestPreferredLanguage(t *testing.T) {
	for _, tc := range []struct{ cookie, header, want string }{
		{"de", "en-US,en;q=0.9", "de"},
		{"en", "de-DE,de;q=0.9", "en"},
		{"", "de-DE,de;q=0.9,en;q=0.8", "de"},
		{"", "en-GB,de;q=0.5", "en"},
		{"", "fr-FR,de;q=0.5", "de"},
		{"", "fr-FR", "en"},
		{"", "", "en"},
		{"xx", "de", "de"},
		{`"><script>`, "", "en"},
	} {
		if got := PreferredLanguage(tc.cookie, tc.header); got != tc.want {
			t.Errorf("PreferredLanguage(%q, %q) = %q, want %q", tc.cookie, tc.header, got, tc.want)
		}
	}
}

func TestShippedGermanFileIsValid(t *testing.T) {
	if err := LoadTranslations(filepath.Join("..", "conf", "locale")); err != nil {
		t.Fatal(err)
	}
	if got := Translate("de", "Sign In"); got != "Anmelden" {
		t.Errorf("got %q", got)
	}
}
