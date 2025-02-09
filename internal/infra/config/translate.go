package config

import (
	"encoding/json"
	"github.com/WilsonSayago/initModules"
	"log"

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
}

func GetTranslateInstance() ITranslate {
	instance := initModules.GetInstance("Translate", func() interface{} {
		return &Translate{}
	})
	return instance.(*Translate)
}

func (t *Translate) Start() {
	t.LoadTranslations()
}

func (t *Translate) LoadTranslations() {
	b := i18n.NewBundle(language.English) // Idioma por defecto
	b.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Cargar archivos de traducción
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

func (t *Translate) TranslateByLang(lang string, messageID string, data map[string]interface{}) string {
	localize := i18n.NewLocalizer(t.bundle, lang)

	return localize.MustLocalize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
}
