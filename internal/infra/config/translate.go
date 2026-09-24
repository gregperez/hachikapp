package config

import (
	"encoding/json"
	"log"

	"github.com/WilsonSayago/initModules/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	logrus "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

type ITranslate interface {
	initModules.IProcess
	LoadTranslations()
	TranslateByLang(lang string, messageID string, data map[string]interface{}) string
}

type Translate struct {
	bundle *i18n.Bundle
}

func GetTranslateInstance() ITranslate {
	return initModules.Once(func() *Translate {
		return &Translate{}
	})
}

func (t *Translate) Start() {
	t.LoadTranslations()
}

func (t *Translate) LoadTranslations() {
	b := i18n.NewBundle(language.English)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)

	_, err := b.LoadMessageFile("internal/resources/locales/en.json")
	if err != nil {
		log.Fatal(err)
	}
	_, err = b.LoadMessageFile("internal/resources/locales/es.json")
	if err != nil {
		log.Fatal(err)
	}

	t.bundle = b
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
		logrus.WithFields(logrus.Fields{
			"message_id": messageID,
			"lang":       lang,
		}).Warn("missing translation")
		return "An error occurred"
	}
	return msg
}
