package chat

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"
	"github.com/ruivalim/google-chat-client/locales"
)

// Os testes conferem as mensagens em português; TestErrorsInEnglish confere
// o inglês, que é a língua de origem dos catálogos.
func TestMain(m *testing.M) {
	i18n.SetLocaleFunc(func() string { return "pt-BR" })
	os.Exit(m.Run())
}

func TestErrorsInEnglish(t *testing.T) {
	en := locales.In("en")
	if got := en.ErrSessionExpired(); !strings.Contains(got, "session expired") {
		t.Fatalf("inglês: %q", got)
	}
	if got := en.UnreadTooltip(3); !strings.Contains(got, "3 unread messages") {
		t.Fatalf("plural em inglês: %q", got)
	}
	if got := locales.In("pt-BR").UnreadTooltip(1); !strings.Contains(got, "1 mensagem não lida") {
		t.Fatalf("plural em português: %q", got)
	}
}

// Toda mensagem do inglês tem tradução: o catálogo em português não pode cair
// no inglês sem ninguém perceber.
func TestCatalogsHaveTheSameKeys(t *testing.T) {
	read := func(name string) map[string]any {
		data, err := os.ReadFile("../../locales/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	en, pt := read("en.json"), read("pt-BR.json")
	for k := range en {
		if strings.HasPrefix(k, "@") {
			continue
		}
		if _, ok := pt[k]; !ok {
			t.Errorf("pt-BR sem %q", k)
		}
	}
	for k := range pt {
		if _, ok := en[k]; !ok && !strings.HasPrefix(k, "@") {
			t.Errorf("pt-BR tem %q, que o inglês não tem", k)
		}
	}
}
