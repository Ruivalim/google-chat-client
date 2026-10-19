package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type memoryVault struct {
	mu     sync.Mutex
	values map[string][]byte
}

func (v *memoryVault) Get(key string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	value, ok := v.values[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (v *memoryVault) Set(key string, value []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.values[key] = append([]byte(nil), value...)
	return nil
}

func (v *memoryVault) Delete(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.values, key)
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func testService(vault Vault, transport http.RoundTripper) *Service {
	return &Service{
		vault:    vault,
		http:     &http.Client{Transport: transport, Timeout: time.Second},
		apiURL:   "https://chat.googleapis.com/v1",
		sessions: make(map[string]*session),
		config:   config{Accounts: []Account{}},
	}
}

func TestAPIUsesTokenForRequestedAccount(t *testing.T) {
	var got []string
	s := testService(&memoryVault{values: map[string][]byte{}}, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		got = append(got, r.Header.Get("Authorization"))
		return jsonResponse(http.StatusOK, `{}`), nil
	}))
	s.sessions["acct-a"] = &session{token: &oauth2.Token{AccessToken: "token-a", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}}
	s.sessions["acct-b"] = &session{token: &oauth2.Token{AccessToken: "token-b", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}}

	for _, id := range []string{"acct-a", "acct-b"} {
		if err := s.api(context.Background(), id, http.MethodGet, "spaces", url.Values{"pageSize": {"100"}}, nil, &struct{}{}); err != nil {
			t.Fatalf("api(%q): %v", id, err)
		}
	}
	if strings.Join(got, ",") != "Bearer token-a,Bearer token-b" {
		t.Fatalf("authorization headers = %v", got)
	}
}

func TestRemoveAndReconnectAccountReplacesSession(t *testing.T) {
	vault := &memoryVault{values: map[string][]byte{"refresh:acct-a": []byte("old-refresh")}}
	s := testService(vault, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{}`), nil
	}))
	s.path = t.TempDir() + "/accounts.json"
	s.config = config{ClientID: "client.apps.googleusercontent.com", Accounts: []Account{{ID: "acct-a", Email: "a@example.com"}}, ActiveAccountID: "acct-a"}
	s.sessions["acct-a"] = &session{token: &oauth2.Token{AccessToken: "old-token", RefreshToken: "old-refresh", Expiry: time.Now().Add(time.Hour)}}

	if err := s.RemoveAccount("acct-a"); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	if _, err := s.accessToken(context.Background(), "acct-a"); err == nil {
		t.Fatal("accessToken succeeded after removal")
	}
	if err := s.attachAccount(Account{ID: "acct-a", Email: "a@example.com"}, &oauth2.Token{
		AccessToken: "new-token", RefreshToken: "new-refresh", Expiry: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("attachAccount: %v", err)
	}
	token, err := s.accessToken(context.Background(), "acct-a")
	if err != nil {
		t.Fatalf("accessToken after reconnect: %v", err)
	}
	if token != "new-token" {
		t.Fatalf("access token = %q, want new session token", token)
	}
	refresh, err := vault.Get("refresh:acct-a")
	if err != nil || string(refresh) != "new-refresh" {
		t.Fatalf("stored refresh token = %q, err = %v", refresh, err)
	}
}

func TestSendMessagePreservesRetryRequestID(t *testing.T) {
	const id = "5f9c92ad-7107-41f1-b1f3-236a234f1839"
	var gotID, gotText string
	s := testService(&memoryVault{values: map[string][]byte{}}, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotID = r.URL.Query().Get("requestId")
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		gotText = body.Text
		return jsonResponse(http.StatusOK, `{"name":"spaces/s1/messages/m1","text":"hello"}`), nil
	}))
	s.sessions["acct-a"] = &session{token: &oauth2.Token{AccessToken: "token", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}}

	if _, err := s.SendMessage(context.Background(), "acct-a", "spaces/s1", "hello", "", id); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if gotID != id || gotText != "hello" {
		t.Fatalf("requestId=%q text=%q", gotID, gotText)
	}
}

func TestSendMessageRejectsInvalidConversationBeforeRequest(t *testing.T) {
	called := false
	s := testService(&memoryVault{values: map[string][]byte{}}, roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return jsonResponse(http.StatusOK, `{}`), nil
	}))
	_, err := s.SendMessage(context.Background(), "acct-a", "spaces/s1?x=1", "hello", "", "")
	if err == nil {
		t.Fatal("SendMessage accepted an invalid conversation identifier")
	}
	if called {
		t.Fatal("invalid conversation reached the transport")
	}
}
