package i18n

import (
	"strings"
	"testing"
)

func TestAcceptLanguage(t *testing.T) {
	for _, tt := range []struct {
		header string
		want   Locale
	}{
		{"", LocaleRU}, {"ru", LocaleRU}, {"ru-RU", LocaleRU}, {"kk", LocaleKK}, {"kk-KZ", LocaleKK}, {"en", LocaleEN}, {"en-US", LocaleEN}, {"en-GB", LocaleEN}, {"fr", LocaleRU}, {"*", LocaleRU},
		{"fr-FR, kk-KZ;q=0.8, en;q=0.5", LocaleKK}, {" EN-us ; q=1 ", LocaleEN}, {"en;q=0,kk", LocaleKK}, {"en;q=invalid,kk", LocaleKK}, {"en;q=NaN,kk", LocaleKK}, {"en;q=1.1,ru", LocaleRU},
	} {
		if got := ParseAcceptLanguage(tt.header); got != tt.want {
			t.Errorf("%q: %s, want %s", tt.header, got, tt.want)
		}
	}
}

func TestKnownLabelsAndCompleteMessages(t *testing.T) {
	for _, table := range []map[string][3]string{messages, reasons, languages, formats} {
		for key, values := range table {
			for _, v := range values {
				if strings.TrimSpace(v) == "" {
					t.Errorf("missing translation: %s", key)
				}
			}
		}
	}
	for _, l := range []Locale{LocaleRU, LocaleKK, LocaleEN} {
		if LanguageLabel(l, "русский") == "" || FormatLabel(l, "свадьба") == "" {
			t.Fatal("missing label")
		}
		if ReasonLabel(l, "UNKNOWN") != "UNKNOWN" {
			t.Fatal("unknown code changed")
		}
	}
	if LanguageLabel(LocaleEN, " РУССКИЙ ") != "Russian" || FormatLabel(LocaleKK, "СВАДЬБА") != "үйлену тойы" {
		t.Fatal("display normalization")
	}
	if Message(Locale("fr"), "required") != Message(LocaleRU, "required") {
		t.Fatal("fallback")
	}
	raw := "Источник.  "
	if ProfileNote(LocaleEN, raw) != "Original profile note: "+raw || ProfileNote(LocaleKK, raw) != "Профильдегі бастапқы мәтін: "+raw {
		t.Fatal("source text changed")
	}
}
