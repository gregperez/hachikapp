package services

import (
	"errors"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

func santiago(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatalf("load tz: %v", err)
	}
	return loc
}

func baseDelivery(kind domain.NotificationKind) domain.Delivery {
	return domain.Delivery{
		ID:        "d1",
		ToEmail:   "ana@example.com",
		OwnerName: "Ana María Pérez",
		Notification: domain.Notification{
			ID:    "n1",
			PetID: "pet-9",
			Kind:  kind,
			Payload: domain.NotificationPayload{
				PetName:   "Luna",
				TagCode:   "ABC123",
				ScannedAt: time.Date(2026, 10, 23, 15, 4, 0, 0, time.UTC),
				Device:    "ios",
			},
		},
	}
}

func TestRenderScanEmail(t *testing.T) {
	d := baseDelivery(domain.NotificationTagScanned)
	d.Notification.Payload.Location = &domain.ScanLocation{Lat: -33.45, Lng: -70.66, AccuracyM: 20, At: d.Notification.Payload.ScannedAt}
	d.Notification.Payload.Message = &domain.TagMessageSummary{Body: "La encontré en la plaza", FinderName: "Pedro", FinderPhone: "+56912345678"}
	d.Notification.Payload.PhoneRevealed = true

	m := RenderNotificationEmail(d, "https://app.hachikapp.cl", santiago(t))

	if m.To != "ana@example.com" {
		t.Fatalf("To = %q", m.To)
	}
	if m.Subject != "Alguien escaneó el Hachikapp Tag de Luna" {
		t.Fatalf("Subject = %q", m.Subject)
	}
	link := "https://app.hachikapp.cl/dashboard/pets/pet-9"
	maps := "https://maps.google.com/?q=-33.450000,-70.660000"
	for name, body := range map[string]string{"text": m.Text, "html": m.HTML} {
		for _, want := range []string{
			"Hola Ana:", "Luna", "23/10/2026 a las 12:04", "un iPhone o iPad", maps,
			"La encontré en la plaza", "Pedro", "56912345678",
			"También vio tu número de teléfono.", link, "Si fuiste tú, puedes ignorar este correo.",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s missing %q\n%s", name, want, body)
			}
		}
	}
	if strings.Contains(m.Text, "María") || strings.Contains(m.HTML, "María") {
		t.Error("greeting must use first name only")
	}
	if !strings.Contains(m.HTML, "Ver en Hachikapp") {
		t.Error("html missing button text")
	}
	if strings.Contains(m.HTML, "tel:") {
		t.Error("finder phone must not be a link")
	}
	for _, c := range []string{"#faf5e9", "#2f3933", "#385d50", "#624536", "560px"} {
		if !strings.Contains(m.HTML, c) {
			t.Errorf("html missing style %s", c)
		}
	}
}

func TestRenderScanEmailMinimal(t *testing.T) {
	d := baseDelivery(domain.NotificationTagScanned)
	d.OwnerName = "  "
	d.Notification.Payload.Device = "other"
	m := RenderNotificationEmail(d, "https://x.test", santiago(t))
	if !strings.Contains(m.Text, "Hola:") || !strings.Contains(m.HTML, "Hola:") {
		t.Errorf("empty name greeting wrong: %q", m.Text)
	}
	if strings.Contains(m.Text, "maps.google.com") || strings.Contains(m.Text, "teléfono") {
		t.Error("must not render absent location/phone")
	}
	if !strings.Contains(m.Text, "otro dispositivo") {
		t.Error("device other missing")
	}
	for dev, want := range map[string]string{"android": "un Android", "desktop": "un computador"} {
		d.Notification.Payload.Device = dev
		if got := RenderNotificationEmail(d, "https://x.test", santiago(t)).Text; !strings.Contains(got, want) {
			t.Errorf("%s: missing %q", dev, want)
		}
	}
}

func TestRenderEscapesFinderContent(t *testing.T) {
	d := baseDelivery(domain.NotificationFinderMessage)
	d.Notification.Payload.Message = &domain.TagMessageSummary{
		Body: "<script>alert(1)</script>", FinderName: "<b>Eve</b>", FinderPhone: "\"><img src=x>",
	}
	d.Notification.Payload.PetName = "<i>Luna</i>"
	m := RenderNotificationEmail(d, "https://x.test", santiago(t))
	if !strings.Contains(m.HTML, "&lt;script&gt;") {
		t.Errorf("html not escaped: %s", m.HTML)
	}
	for _, bad := range []string{"<script>", "<b>Eve", "<img", "<i>Luna"} {
		if strings.Contains(m.HTML, bad) {
			t.Errorf("html contains raw %q", bad)
		}
	}
}

func TestRenderSubjectsByKind(t *testing.T) {
	cases := map[domain.NotificationKind]string{
		domain.NotificationTagScanned:     "Alguien escaneó el Hachikapp Tag de Luna",
		domain.NotificationFinderMessage:  "Luna: te dejaron un mensaje",
		domain.NotificationLocationShared: "Luna: compartieron dónde la encontraron",
		domain.NotificationPhoneRevealed:  "Luna: alguien vio tu número",
	}
	for kind, want := range cases {
		m := RenderNotificationEmail(baseDelivery(kind), "https://x.test", santiago(t))
		if m.Subject != want {
			t.Errorf("%s: subject = %q, want %q", kind, m.Subject, want)
		}
		if !strings.Contains(m.Text, "23/10/2026 a las 12:04") {
			t.Errorf("%s: missing scan time", kind)
		}
	}
}

func TestRenderNilLocationFallsBackToUTC(t *testing.T) {
	m := RenderNotificationEmail(baseDelivery(domain.NotificationTagScanned), "https://x.test", nil)
	if !strings.Contains(m.Text, "23/10/2026 a las 15:04") {
		t.Errorf("text = %s", m.Text)
	}
}

func TestRenderHTMLFallbackOnTemplateError(t *testing.T) {
	bad := template.Must(template.New("bad").Funcs(template.FuncMap{"boom": func() (string, error) { return "", errors.New("boom") }}).Parse(`{{boom}}`))
	text := "Hola <Ana>: & más"
	got := renderHTML(bad, emailView{}, text)
	if !strings.Contains(got, "Hola &lt;Ana&gt;: &amp; más") || !strings.Contains(got, "<pre") {
		t.Errorf("fallback = %q", got)
	}
}

func TestRenderOmitsEmptyFinderFieldsAndZeroAccuracy(t *testing.T) {
	d := baseDelivery(domain.NotificationFinderMessage)
	d.Notification.Payload.Message = &domain.TagMessageSummary{Body: "Hola"}
	d.Notification.Payload.Location = &domain.ScanLocation{Lat: 1, Lng: 2}
	m := RenderNotificationEmail(d, "https://x.test", santiago(t))
	for name, s := range map[string]string{"text": m.Text, "html": m.HTML} {
		for _, bad := range []string{"Nombre:", "Teléfono:", "precisión"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s contains %q", name, bad)
			}
		}
	}
}
