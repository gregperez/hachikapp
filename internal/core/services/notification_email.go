package services

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

type emailView struct {
	Greeting     string
	Intro        string
	When         string
	Device       string
	HasLocation  bool
	MapsURL      string
	HasMessage   bool
	Body         string
	FinderName   string
	FinderPhone  string
	PhoneLine    string
	PetURL       string
	Closing      string
	ButtonLabel  string
	LocationLine string
}

var emailTemplate = template.Must(template.New("email").Parse(`<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body style="margin:0;padding:24px 12px;background:#faf5e9;color:#2f3933;font-family:Helvetica,Arial,sans-serif;font-size:16px;line-height:1.5;">
<div style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:12px;padding:24px;">
<p style="margin:0 0 12px;">{{.Greeting}}</p>
<p style="margin:0 0 12px;">{{.Intro}}</p>
<p style="margin:0 0 12px;color:#624536;">Fue {{.When}}{{if .Device}}, desde {{.Device}}{{end}}.</p>
{{if .HasLocation}}<p style="margin:0 0 12px;">{{.LocationLine}} <a href="{{.MapsURL}}" style="color:#385d50;">Ver en el mapa</a></p>
{{end}}{{if .HasMessage}}<div style="margin:0 0 12px;padding:12px;background:#faf5e9;border-radius:8px;">
<p style="margin:0 0 8px;">{{.Body}}</p>
{{if .FinderName}}<p style="margin:0;color:#624536;">Nombre: {{.FinderName}}</p>
{{end}}{{if .FinderPhone}}<p style="margin:0;color:#624536;">Teléfono: {{.FinderPhone}}</p>
{{end}}
</div>
{{end}}{{if .PhoneLine}}<p style="margin:0 0 12px;">{{.PhoneLine}}</p>
{{end}}<p style="margin:16px 0;"><a href="{{.PetURL}}" style="display:inline-block;background:#385d50;color:#ffffff;text-decoration:none;padding:12px 20px;border-radius:8px;">{{.ButtonLabel}}</a></p>
<p style="margin:0;color:#624536;font-size:14px;">{{.Closing}}</p>
</div></body></html>
`))

func deviceLabel(code string) string {
	switch code {
	case "ios":
		return "un iPhone o iPad"
	case "android":
		return "un Android"
	case "desktop":
		return "un computador"
	case "other":
		return "otro dispositivo"
	}
	return ""
}

func emailSubject(kind domain.NotificationKind, pet string) string {
	switch kind {
	case domain.NotificationFinderMessage:
		return pet + ": te dejaron un mensaje"
	case domain.NotificationLocationShared:
		return pet + ": compartieron dónde la encontraron"
	case domain.NotificationPhoneRevealed:
		return pet + ": alguien vio tu número"
	default:
		return "Alguien escaneó el Hachikapp Tag de " + pet
	}
}

// RenderNotificationEmail builds the plain text and HTML email for a delivery.
// baseURL must not end with a slash; a nil loc renders times in UTC.
func RenderNotificationEmail(d domain.Delivery, baseURL string, loc *time.Location) domain.Email {
	if loc == nil {
		loc = time.UTC
	}
	p := d.Notification.Payload
	greeting := "Hola:"
	if first := domain.FirstName(d.OwnerName); first != "" {
		greeting = "Hola " + first + ":"
	}
	v := emailView{
		Greeting:    greeting,
		Intro:       fmt.Sprintf("alguien encontró a %s y escaneó su placa.", p.PetName),
		When:        "el " + p.ScannedAt.In(loc).Format("02/01/2006") + " a las " + p.ScannedAt.In(loc).Format("15:04"),
		Device:      deviceLabel(p.Device),
		PetURL:      baseURL + "/dashboard/pets/" + d.Notification.PetID,
		Closing:     "Si fuiste tú, puedes ignorar este correo.",
		ButtonLabel: "Ver en Hachikapp",
	}
	v.Intro = capitalizeFirst(v.Intro)
	if p.Location != nil {
		v.HasLocation = true
		v.MapsURL = fmt.Sprintf("https://maps.google.com/?q=%.6f,%.6f", p.Location.Lat, p.Location.Lng)
		v.LocationLine = "Compartieron dónde la encontraron."
		if p.Location.AccuracyM > 0 {
			v.LocationLine = fmt.Sprintf("Compartieron dónde la encontraron (precisión aproximada de %d m).", p.Location.AccuracyM)
		}
	}
	if p.Message != nil {
		v.HasMessage = true
		v.Body = p.Message.Body
		v.FinderName = p.Message.FinderName
		v.FinderPhone = p.Message.FinderPhone
	}
	if p.PhoneRevealed {
		v.PhoneLine = "También vio tu número de teléfono."
	}

	text := renderText(v)
	return domain.Email{
		To:      d.ToEmail,
		Subject: emailSubject(d.Notification.Kind, p.PetName),
		Text:    text,
		HTML:    renderHTML(emailTemplate, v, text),
	}
}

// renderHTML executes tpl; if that fails or yields nothing it falls back to
// the escaped text part so the email always carries its content.
func renderHTML(tpl *template.Template, v emailView, text string) string {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, v); err != nil || buf.Len() == 0 {
		return `<pre style="font-family:inherit;white-space:pre-wrap">` + template.HTMLEscapeString(text) + `</pre>`
	}
	return buf.String()
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func renderText(v emailView) string {
	var b strings.Builder
	b.WriteString(v.Greeting + " " + lowerFirst(v.Intro) + "\n\n")
	b.WriteString("Fue " + v.When)
	if v.Device != "" {
		b.WriteString(", desde " + v.Device)
	}
	b.WriteString(".\n\n")
	if v.HasLocation {
		b.WriteString(v.LocationLine + "\n" + v.MapsURL + "\n\n")
	}
	if v.HasMessage {
		b.WriteString(v.Body + "\n")
		if v.FinderName != "" {
			b.WriteString("Nombre: " + v.FinderName + "\n")
		}
		if v.FinderPhone != "" {
			b.WriteString("Teléfono: " + v.FinderPhone + "\n")
		}
		b.WriteString("\n")
	}
	if v.PhoneLine != "" {
		b.WriteString(v.PhoneLine + "\n\n")
	}
	b.WriteString(v.ButtonLabel + ": " + v.PetURL + "\n\n")
	b.WriteString(v.Closing + "\n")
	return b.String()
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToLower(string(r[0])) + string(r[1:])
}
