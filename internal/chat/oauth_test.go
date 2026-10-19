package chat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

const testClientID = "123-abc.apps.googleusercontent.com"

// googleStub responde pelos endpoints de token e userinfo do Google.
type googleStub struct {
	scope     string
	refresh   string
	userinfo  string
	tokenHits atomic.Int32
	mu        sync.Mutex
	clients   []string // client_id de cada pedido de token, na ordem
}

func (g *googleStub) clientIDs() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.clients)
}

func (g *googleStub) RoundTrip(r *http.Request) (*http.Response, error) {
	switch r.URL.Host + r.URL.Path {
	case "oauth2.googleapis.com/token":
		g.tokenHits.Add(1)
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		g.mu.Lock()
		g.clients = append(g.clients, form.Get("client_id"))
		g.mu.Unlock()
		if form.Get("grant_type") == "authorization_code" && (form.Get("code") != "good-code" || form.Get("code_verifier") == "") {
			return jsonResponse(http.StatusBadRequest, `{"error":"invalid_grant"}`), nil
		}
		res := map[string]any{"access_token": "access-1", "token_type": "Bearer", "expires_in": 3600, "scope": g.scope}
		if g.refresh != "" {
			res["refresh_token"] = g.refresh
		}
		data, _ := json.Marshal(res)
		resp := jsonResponse(http.StatusOK, string(data))
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	case "openidconnect.googleapis.com/v1/userinfo":
		if r.Header.Get("Authorization") != "Bearer access-1" {
			return jsonResponse(http.StatusUnauthorized, `{}`), nil
		}
		return jsonResponse(http.StatusOK, g.userinfo), nil
	}
	return jsonResponse(http.StatusNotFound, `{}`), nil
}

func allScopes() string { return strings.Join(scopes, " ") }

func configuredService(t *testing.T, g *googleStub) (*Service, *memoryVault) {
	t.Helper()
	vault := &memoryVault{values: map[string][]byte{}}
	s := testService(vault, g)
	s.path = t.TempDir() + "/accounts.json"
	if err := s.ImportProject(Credentials{ClientID: testClientID, ClientSecret: "segredo", ProjectID: "chat-teste"}); err != nil {
		t.Fatalf("ImportProject: %v", err)
	}
	return s, vault
}

// connect registra uma conta já conectada pelo projeto clientID.
func connect(s *Service, id, clientID string, x *session) {
	s.config.Accounts = append(s.config.Accounts, Account{ID: id, Email: id + "@example.com", ClientID: clientID})
	s.sessions[id] = x
}

// browser simula o navegador do sistema: lê a URL de consentimento e chama o callback local.
func browser(t *testing.T, query func(state string) url.Values, status chan<- int) func(string) error {
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		q := u.Query()
		if q.Get("code_challenge_method") != "S256" || q.Get("access_type") != "offline" {
			t.Errorf("consent URL sem PKCE ou acesso offline: %s", raw)
		}
		if !strings.HasPrefix(q.Get("redirect_uri"), "http://127.0.0.1:") {
			t.Errorf("redirect_uri fora do loopback: %s", q.Get("redirect_uri"))
		}
		go func() {
			resp, err := http.Get(q.Get("redirect_uri") + "?" + query(q.Get("state")).Encode())
			if err != nil {
				t.Errorf("callback: %v", err)
				return
			}
			resp.Body.Close()
			if status != nil {
				status <- resp.StatusCode
			}
		}()
		return nil
	}
}

func TestAddAccountStoresSessionInVaultNotInConfig(t *testing.T) {
	g := &googleStub{scope: allScopes(), refresh: "refresh-1", userinfo: `{"sub":"1001","name":"Ana","email":"ana@empresa.com","email_verified":true,"hd":"empresa.com"}`}
	s, vault := configuredService(t, g)

	account, err := s.AddAccount(context.Background(), testClientID, browser(t, func(state string) url.Values {
		return url.Values{"state": {state}, "code": {"good-code"}}
	}, nil))
	if err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	if account.ID != "1001" || account.Domain != "empresa.com" {
		t.Fatalf("account = %+v", account)
	}
	if got, _ := vault.Get("refresh:1001"); string(got) != "refresh-1" {
		t.Fatalf("refresh token no chaveiro = %q", got)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "refresh-1") || strings.Contains(string(data), "segredo") || strings.Contains(string(data), "access-1") {
		t.Fatalf("accounts.json vazou credencial: %s", data)
	}
	if info, _ := os.Stat(s.path); info.Mode().Perm() != 0o600 {
		t.Fatalf("accounts.json com permissão %v", info.Mode().Perm())
	}
	if st := s.State(); st.ActiveAccountID != "1001" || len(st.Accounts) != 1 {
		t.Fatalf("state = %+v", st)
	}
	reopened, err := Open(s.path, vault)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if st := reopened.State(); st.ActiveAccountID != "1001" || st.Projects[0].ProjectID != "chat-teste" || st.Accounts[0].ClientID != testClientID {
		t.Fatalf("reaberto = %+v", st)
	}
}

func TestAddAccountIgnoresCallbackWithWrongState(t *testing.T) {
	g := &googleStub{scope: allScopes(), refresh: "r", userinfo: `{}`}
	s, _ := configuredService(t, g)
	status := make(chan int, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := s.AddAccount(ctx, testClientID, browser(t, func(string) url.Values {
		return url.Values{"state": {"forjado"}, "code": {"good-code"}}
	}, status))
	if err == nil {
		t.Fatal("login concluído com state forjado")
	}
	if code := <-status; code != http.StatusBadRequest {
		t.Fatalf("callback forjado respondeu %d", code)
	}
	if g.tokenHits.Load() != 0 {
		t.Fatal("código de state forjado foi trocado por token")
	}
}

func TestAddAccountFailures(t *testing.T) {
	cases := []struct {
		name     string
		g        *googleStub
		query    func(string) url.Values
		contains string
	}{
		{"usuário negou", &googleStub{scope: allScopes()}, func(st string) url.Values {
			return url.Values{"state": {st}, "error": {"access_denied"}}
		}, "não foi autorizado"},
		{"admin bloqueou", &googleStub{scope: allScopes()}, func(st string) url.Values {
			return url.Values{"state": {st}, "error": {"admin_policy_enforced"}}
		}, "administrador"},
		{"projeto interno", &googleStub{scope: allScopes()}, func(st string) url.Values {
			return url.Values{"state": {st}, "error": {"org_internal"}}
		}, "Externo"},
		{"escopo de chat faltando", &googleStub{scope: "openid email profile", refresh: "r"}, func(st string) url.Values {
			return url.Values{"state": {st}, "code": {"good-code"}}
		}, "permissão de Chat"},
		{"sem refresh token", &googleStub{scope: allScopes(), userinfo: `{"sub":"1","email":"a@b.c","email_verified":true}`}, func(st string) url.Values {
			return url.Values{"state": {st}, "code": {"good-code"}}
		}, "sessão persistente"},
		{"email não verificado", &googleStub{scope: allScopes(), refresh: "r", userinfo: `{"sub":"1","email":"a@b.c","email_verified":false}`}, func(st string) url.Values {
			return url.Values{"state": {st}, "code": {"good-code"}}
		}, "identidade"},
		{"sub com caractere inválido", &googleStub{scope: allScopes(), refresh: "r", userinfo: `{"sub":"../x","email":"a@b.c","email_verified":true}`}, func(st string) url.Values {
			return url.Values{"state": {st}, "code": {"good-code"}}
		}, "identidade"},
		{"código recusado", &googleStub{scope: allScopes(), refresh: "r"}, func(st string) url.Values {
			return url.Values{"state": {st}, "code": {"bad-code"}}
		}, "não concluiu"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, vault := configuredService(t, c.g)
			_, err := s.AddAccount(context.Background(), testClientID, browser(t, c.query, nil))
			if err == nil || !strings.Contains(err.Error(), c.contains) {
				t.Fatalf("err = %v, want contendo %q", err, c.contains)
			}
			if len(s.State().Accounts) != 0 {
				t.Fatal("conta registrada mesmo com falha")
			}
			for k := range vault.values {
				if strings.HasPrefix(k, "refresh:") {
					t.Fatalf("sessão gravada mesmo com falha: %s", k)
				}
			}
		})
	}
}

func TestAddAccountRejectsSecondLoginWhileFirstRuns(t *testing.T) {
	s, _ := configuredService(t, &googleStub{})
	opened := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := s.AddAccount(ctx, testClientID, func(string) error { close(opened); return nil })
		done <- err
	}()
	<-opened
	if _, err := s.AddAccount(context.Background(), testClientID, func(string) error { return nil }); err == nil {
		t.Fatal("segundo login aceito em paralelo")
	}
	cancel()
	if err := <-done; err == nil || !strings.Contains(err.Error(), "cancelado") {
		t.Fatalf("cancelamento: %v", err)
	}
}

func TestAccessTokenRefreshesOnceUnderConcurrency(t *testing.T) {
	g := &googleStub{scope: allScopes()}
	s, vault := configuredService(t, g)
	vault.values["refresh:acct-a"] = []byte("stored-refresh")
	connect(s, "acct-a", testClientID, &session{})

	var wg sync.WaitGroup
	tokens := make([]string, 20)
	errs := make([]error, 20)
	for i := range tokens {
		wg.Go(func() { tokens[i], errs[i] = s.accessToken(context.Background(), "acct-a") })
	}
	wg.Wait()
	for i := range tokens {
		if errs[i] != nil || tokens[i] != "access-1" {
			t.Fatalf("goroutine %d: token=%q err=%v", i, tokens[i], errs[i])
		}
	}
	if n := g.tokenHits.Load(); n != 1 {
		t.Fatalf("renovações = %d, want 1", n)
	}
}

func TestAccessTokenKeepsRotatedRefreshToken(t *testing.T) {
	g := &googleStub{scope: allScopes(), refresh: "rotated"}
	s, vault := configuredService(t, g)
	vault.values["refresh:acct-a"] = []byte("old")
	connect(s, "acct-a", testClientID, &session{token: &oauth2.Token{AccessToken: "expired", RefreshToken: "old", Expiry: time.Now().Add(-time.Minute)}})
	if _, err := s.accessToken(context.Background(), "acct-a"); err != nil {
		t.Fatal(err)
	}
	if got, _ := vault.Get("refresh:acct-a"); string(got) != "rotated" {
		t.Fatalf("refresh no chaveiro = %q", got)
	}
}

func TestAccessTokenWithoutStoredSessionAsksToReconnect(t *testing.T) {
	s, _ := configuredService(t, &googleStub{})
	connect(s, "acct-a", testClientID, &session{})
	if _, err := s.accessToken(context.Background(), "acct-a"); err == nil || !strings.Contains(err.Error(), "conecte") {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.accessToken(context.Background(), "desconhecida"); err == nil {
		t.Fatal("token para conta não conectada")
	}
}

func TestImportProjectValidation(t *testing.T) {
	s, vault := configuredService(t, &googleStub{})
	for _, c := range []Credentials{
		{ClientID: "qualquer-coisa", ClientSecret: "s"},
		{ClientID: "a b.apps.googleusercontent.com", ClientSecret: "s"},
		{ClientID: testClientID, ClientSecret: "  "},
	} {
		if err := s.ImportProject(c); err == nil {
			t.Errorf("ImportProject(%+v) aceito", c)
		}
	}
	connect(s, "1", testClientID, &session{})
	if err := s.ImportProject(Credentials{ClientID: "outro.apps.googleusercontent.com", ClientSecret: "s"}); err != nil {
		t.Fatalf("segundo projeto com conta conectada no primeiro: %v", err)
	}
	if err := s.ImportProject(Credentials{ClientID: " " + testClientID + "\n", ClientSecret: "novo", ProjectID: "renomeado"}); err != nil {
		t.Fatalf("reimportar o mesmo projeto: %v", err)
	}
	st := s.State()
	if len(st.Projects) != 2 || st.Projects[0].ProjectID != "renomeado" {
		t.Fatalf("projetos = %+v", st.Projects)
	}
	if c, _ := s.credentials(testClientID); c.ClientSecret != "novo" {
		t.Fatalf("secret depois de reimportar = %q", c.ClientSecret)
	}
	if _, ok := vault.values[projectKey("outro.apps.googleusercontent.com")]; !ok {
		t.Fatal("segundo projeto não foi para o chaveiro")
	}
}

func TestOpenRejectsBrokenConfig(t *testing.T) {
	dir := t.TempDir()
	vault := &memoryVault{values: map[string][]byte{}}
	if s, err := Open(dir+"/nao-existe.json", vault); err != nil || len(s.State().Projects) != 0 {
		t.Fatalf("config ausente: %v", err)
	}
	for name, body := range map[string]string{
		"truncado":       `{"clientId": "x", "accounts": [`,
		"conta inválida": `{"accounts":[{"id":"../../etc","email":"a@b.c"}]}`,
	} {
		path := dir + "/" + strings.ReplaceAll(name, " ", "-") + ".json"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(path, vault); err == nil {
			t.Errorf("%s: Open aceitou", name)
		}
	}
}

func TestAccessTokenTellsExpiredSessionApartFromNetwork(t *testing.T) {
	cases := map[string]struct {
		rt   roundTripFunc
		want string
	}{
		"sessão vencida": {func(*http.Request) (*http.Response, error) {
			resp := jsonResponse(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}, "expirou"},
		"sem rede": {func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp: no route to host")
		}, "confira a conexão"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			vault := &memoryVault{values: map[string][]byte{"refresh:acct-a": []byte("r")}}
			data, _ := json.Marshal(Credentials{ClientID: testClientID, ClientSecret: "s"})
			vault.values[projectKey(testClientID)] = data
			s := testService(vault, c.rt)
			connect(s, "acct-a", testClientID, &session{})
			_, err := s.accessToken(context.Background(), "acct-a")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want contendo %q", err, c.want)
			}
		})
	}
}
