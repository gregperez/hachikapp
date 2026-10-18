package config

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/WilsonSayago/initModules/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type ITranslate interface {
	initModules.IProcess
	LoadTranslations()
	TranslateByLang(lang string, messageID string, data map[string]interface{}) string
}

type Translate struct {
	bundle *i18n.Bundle
	files  []string
	logger *slog.Logger
}

// NewTranslate constructs a translator without loading files or starting work.
func NewTranslate(logger *slog.Logger, files ...string) *Translate {
	if len(files) == 0 {
		files = []string{
			"internal/resources/locales/en.json",
			"internal/resources/locales/es.json",
		}
	}
	return &Translate{files: files, logger: logger}
}

// GetTranslateInstance is retained for compatibility with leftover callers.
//
// Deprecated: use NewTranslate.
func GetTranslateInstance() ITranslate {
	return NewTranslate(nil)
}

func (t *Translate) Start() {
	if err := t.LoadTranslationsFiles(); err != nil && t.logger != nil {
		t.logger.Error("load translations", "err", err)
	}
}

func (t *Translate) LoadTranslations() {
	_ = t.LoadTranslationsFiles()
}

// LoadTranslationsFiles loads locale files and returns an error instead of exiting.
func (t *Translate) LoadTranslationsFiles() error {
	if t == nil {
		return fmt.Errorf("translator is nil")
	}
	b := i18n.NewBundle(language.English)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, file := range t.files {
		if _, err := b.LoadMessageFile(file); err != nil {
			return fmt.Errorf("load translation file %s: %w", file, err)
		}
	}
	t.bundle = b
	return nil
}

// TranslateByLang looks up a message without panicking.
// Missing translation IDs log the technical ID and return a generic message.
func (t *Translate) TranslateByLang(lang string, messageID string, data map[string]interface{}) string {
	if t == nil || t.bundle == nil || messageID == "" {
		return "An error occurred"
	}
	localize := i18n.NewLocalizer(t.bundle, lang)
	msg, err := localize.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("missing translation",
				"message_id", messageID,
				"lang", lang,
			)
		}
		return "An error occurred"
	}
	return msg
}
