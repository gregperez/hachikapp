package config

import (
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func TestTranslateByLang_MissingIDDoesNotPanic(t *testing.T) {
	tr := &Translate{bundle: i18n.NewBundle(language.English)}
	msg := tr.TranslateByLang("en", "errors.does-not-exist", nil)
	if msg != "An error occurred" {
		t.Fatalf("got %q", msg)
	}
}

func TestTranslateByLang_EmptyID(t *testing.T) {
	tr := &Translate{bundle: i18n.NewBundle(language.English)}
	if got := tr.TranslateByLang("en", "", nil); got != "An error occurred" {
		t.Fatalf("got %q", got)
	}
}
