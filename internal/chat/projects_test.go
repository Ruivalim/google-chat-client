package chat

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

const otherClientID = "456-def.apps.googleusercontent.com"

func expired() *session {
	return &session{token: &oauth2.Token{AccessToken: "velho", RefreshToken: "r", Expiry: time.Now().Add(-time.Minute)}}
}

func TestEachAccountRefreshesWithItsOwnProject(t *testing.T) {
	g := &googleStub{scope: allScopes()}
	s, _ := configuredService(t, g)
	if err := s.ImportProject(Credentials{ClientID: otherClientID, ClientSecret: "segredo-b"}); err != nil {
		t.Fatal(err)
	}
	connect(s, "conta-a", testClientID, expired())
	connect(s, "conta-b", otherClientID, expired())

	for _, id := range []string{"conta-b", "conta-a", "conta-b"} {
		if _, err := s.accessToken(context.Background(), id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		s.sessions[id].token.Expiry = time.Now().Add(-time.Minute)
	}
	if got := g.clientIDs(); !slices.Equal(got, []string{otherClientID, testClientID, otherClientID}) {
		t.Fatalf("client_id usado em cada renovação = %v", got)
	}
}

func TestRemoveProjectOnlyWhenNoAccountUsesIt(t *testing.T) {
	s, vault := configuredService(t, &googleStub{})
	if err := s.ImportProject(Credentials{ClientID: otherClientID, ClientSecret: "b"}); err != nil {
		t.Fatal(err)
	}
	vault.values["refresh:conta-b"] = []byte("r")
	connect(s, "conta-b", otherClientID, &session{})

	if err := s.RemoveProject(otherClientID); err == nil || !strings.Contains(err.Error(), "remova as contas") {
		t.Fatalf("removeu projeto em uso: %v", err)
	}
	if err := s.RemoveAccount("conta-b"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveProject(otherClientID); err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}
	if _, ok := vault.values[projectKey(otherClientID)]; ok {
		t.Fatal("secret do projeto removido ficou no chaveiro")
	}
	if st := s.State(); len(st.Projects) != 1 || st.Projects[0].ClientID != testClientID {
		t.Fatalf("projetos = %+v", st.Projects)
	}
	if err := s.RemoveProject("nao-existe.apps.googleusercontent.com"); err == nil {
		t.Fatal("removeu projeto inexistente")
	}
}

func TestAddAccountNeedsImportedProject(t *testing.T) {
	s, _ := configuredService(t, &googleStub{})
	opened := false
	_, err := s.AddAccount(context.Background(), otherClientID, func(string) error { opened = true; return nil })
	if err == nil || opened {
		t.Fatalf("login por projeto não importado: err=%v navegador=%v", err, opened)
	}
}

func TestReconnectThroughAnotherProjectMovesTheAccount(t *testing.T) {
	g := &googleStub{scope: allScopes(), refresh: "novo-refresh", userinfo: `{"sub":"1001","name":"Ana","email":"ana@empresa.com","email_verified":true,"hd":"empresa.com"}`}
	s, vault := configuredService(t, g)
	if err := s.ImportProject(Credentials{ClientID: otherClientID, ClientSecret: "b"}); err != nil {
		t.Fatal(err)
	}
	connect(s, "1001", testClientID, expired())

	if _, err := s.AddAccount(context.Background(), otherClientID, browser(t, func(state string) url.Values {
		return url.Values{"state": {state}, "code": {"good-code"}}
	}, nil)); err != nil {
		t.Fatal(err)
	}
	st := s.State()
	if len(st.Accounts) != 1 || st.Accounts[0].ClientID != otherClientID {
		t.Fatalf("contas = %+v", st.Accounts)
	}
	if got := vault.values["refresh:1001"]; string(got) != "novo-refresh" {
		t.Fatalf("refresh = %q", got)
	}
	if err := s.RemoveProject(testClientID); err != nil {
		t.Fatalf("o projeto antigo ficou sem contas e devia poder sair: %v", err)
	}
}

func TestSingleProjectFormatMigrates(t *testing.T) {
	path := t.TempDir() + "/accounts.json"
	old := `{"clientId":"` + testClientID + `","accounts":[{"id":"conta-a","email":"a@example.com"}],"activeAccountId":"conta-a"}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, _ := json.Marshal(Credentials{ClientID: testClientID, ClientSecret: "antigo"})
	vault := &memoryVault{values: map[string][]byte{"oauth-client": secret, "refresh:conta-a": []byte("r")}}

	s, err := Open(path, vault)
	if err != nil {
		t.Fatal(err)
	}
	st := s.State()
	if len(st.Projects) != 1 || st.Projects[0].ClientID != testClientID || st.Accounts[0].ClientID != testClientID {
		t.Fatalf("estado migrado = %+v", st)
	}
	g := &googleStub{scope: allScopes()}
	s.http.Transport = g
	if _, err := s.accessToken(context.Background(), "conta-a"); err != nil {
		t.Fatalf("renovar com o secret do formato antigo: %v", err)
	}
	if _, ok := vault.values["oauth-client"]; ok {
		t.Fatal("chave antiga continua no chaveiro")
	}
	if _, ok := vault.values[projectKey(testClientID)]; !ok {
		t.Fatal("secret não foi para a chave do projeto")
	}
	if err := s.SelectAccount("conta-a"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if _, legacy := saved["clientId"]; legacy || saved["projects"] == nil {
		t.Fatalf("arquivo salvo ainda no formato antigo: %s", data)
	}
}
