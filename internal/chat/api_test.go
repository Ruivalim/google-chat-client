package chat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func apiService(t *testing.T, rt roundTripFunc) *Service {
	t.Helper()
	s := testService(&memoryVault{values: map[string][]byte{}}, rt)
	s.sessions["acct-a"] = &session{token: &oauth2.Token{AccessToken: "token", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}}
	return s
}

func TestAPIErrorsAreReadable(t *testing.T) {
	cases := map[int]string{
		http.StatusUnauthorized:        "conecte essa conta",
		http.StatusForbidden:           "permissão",
		http.StatusTooManyRequests:     "limitou",
		http.StatusNotFound:            "não está mais disponível",
		http.StatusInternalServerError: "(500)",
	}
	for status, want := range cases {
		s := apiService(t, func(*http.Request) (*http.Response, error) { return jsonResponse(status, `{"error":{}}`), nil })
		_, err := s.Messages(context.Background(), "acct-a", "spaces/s1", "", "")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("status %d: err = %v, want contendo %q", status, err, want)
		}
	}
	s := apiService(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("dial tcp: no route") })
	if _, err := s.Spaces(context.Background(), "acct-a", ""); err == nil || !strings.Contains(err.Error(), "conexão") {
		t.Errorf("erro de rede: %v", err)
	}
	s = apiService(t, func(*http.Request) (*http.Response, error) { return jsonResponse(http.StatusOK, `{"messages": [`), nil })
	if _, err := s.Messages(context.Background(), "acct-a", "spaces/s1", "", ""); err == nil || !strings.Contains(err.Error(), "não pôde ser lida") {
		t.Errorf("JSON truncado: %v", err)
	}
}

func TestMessagesAreOldestFirstAndThreadFiltered(t *testing.T) {
	var query string
	s := apiService(t, func(r *http.Request) (*http.Response, error) {
		query = r.URL.RawQuery
		return jsonResponse(http.StatusOK, `{"messages":[{"name":"spaces/s1/messages/3"},{"name":"spaces/s1/messages/2"},{"name":"spaces/s1/messages/1"}],"nextPageToken":"p2"}`), nil
	})
	page, err := s.Messages(context.Background(), "acct-a", "spaces/s1", "p1", "spaces/s1/threads/t.9")
	if err != nil {
		t.Fatal(err)
	}
	if page.Messages[0].Name != "spaces/s1/messages/1" || page.Messages[2].Name != "spaces/s1/messages/3" || page.NextPageToken != "p2" {
		t.Fatalf("page = %+v", page)
	}
	for _, want := range []string{"orderBy=create_time+DESC", "pageToken=p1", "filter=thread.name+%3D+spaces%2Fs1%2Fthreads%2Ft.9"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q sem %q", query, want)
		}
	}
}

func TestThreadMustBelongToSpace(t *testing.T) {
	called := false
	s := apiService(t, func(*http.Request) (*http.Response, error) {
		called = true
		return jsonResponse(http.StatusOK, `{}`), nil
	})
	for _, thread := range []string{"spaces/outro/threads/t1", "spaces/s1/threads/", "spaces/s1/threads/t1 OR 1", "spaces/s10/threads/t1"} {
		if _, err := s.Messages(context.Background(), "acct-a", "spaces/s1", "", thread); err == nil {
			t.Errorf("Messages aceitou thread %q", thread)
		}
		if _, err := s.SendMessage(context.Background(), "acct-a", "spaces/s1", "oi", thread, ""); err == nil {
			t.Errorf("SendMessage aceitou thread %q", thread)
		}
	}
	if called {
		t.Fatal("thread inválida chegou ao Google")
	}
}

func TestSendMessageLimits(t *testing.T) {
	var body string
	s := apiService(t, func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		if r.URL.Query().Get("messageReplyOption") != "REPLY_MESSAGE_OR_FAIL" {
			t.Errorf("resposta em thread sem REPLY_MESSAGE_OR_FAIL: %s", r.URL.RawQuery)
		}
		return jsonResponse(http.StatusOK, `{"name":"spaces/s1/messages/m"}`), nil
	})
	for _, text := range []string{"", "   \n\t", strings.Repeat("a", 32001)} {
		if _, err := s.SendMessage(context.Background(), "acct-a", "spaces/s1", text, "", ""); err == nil {
			t.Errorf("SendMessage aceitou texto de %d bytes", len(text))
		}
	}
	if _, err := s.SendMessage(context.Background(), "acct-a", "spaces/s1", "x", "", "não-é-uuid"); err == nil {
		t.Error("SendMessage aceitou requestId inválido")
	}
	if _, err := s.SendMessage(context.Background(), "acct-a", "spaces/s1", strings.Repeat("a", 32000), "spaces/s1/threads/t1", ""); err != nil {
		t.Fatalf("texto no limite: %v", err)
	}
	if !strings.Contains(body, `"thread":{"name":"spaces/s1/threads/t1"}`) {
		t.Fatalf("corpo sem thread: %.80s", body)
	}
}

func TestRequestIDIsUUIDv4(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id := requestID()
		if !requestIDPattern.MatchString(id) || id[14] != '4' || !strings.ContainsRune("89ab", rune(id[19])) || seen[id] {
			t.Fatalf("requestID inválido ou repetido: %s", id)
		}
		seen[id] = true
	}
}

func TestAPIErrorsExplainProjectSetup(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   []string
	}{
		{"API desativada", http.StatusForbidden,
			`{"error":{"code":403,"message":"Google Chat API has not been used in project 760952425460 before or it is disabled.","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SERVICE_DISABLED","domain":"googleapis.com"}]}}`,
			[]string{"não está ativada", "has not been used in project 760952425460"}},
		{"escopo faltando", http.StatusForbidden,
			`{"error":{"code":403,"message":"Request had insufficient authentication scopes.","details":[{"reason":"ACCESS_TOKEN_SCOPE_INSUFFICIENT"}]}}`,
			[]string{"escopos", "insufficient authentication scopes"}},
		{"app de Chat sem configuração", http.StatusNotFound,
			`{"error":{"code":404,"message":"Google Chat app not found. To create a Chat app, you must turn on the Chat API and configure the app in the Google Cloud console.","status":"NOT_FOUND"}}`,
			[]string{"aba Configuração", "Chat app not found"}},
		{"403 sem motivo conhecido", http.StatusForbidden,
			`{"error":{"code":403,"message":"The caller does not have permission","status":"PERMISSION_DENIED"}}`,
			[]string{"recusou a permissão", "does not have permission"}},
		{"403 sem corpo", http.StatusForbidden, ``, []string{"recusou a permissão"}},
		{"404 de conversa", http.StatusNotFound, `{"error":{"code":404,"message":"Requested entity was not found."}}`, []string{"não está mais disponível"}},
		{"500 com mensagem", http.StatusInternalServerError, `{"error":{"code":500,"message":"Internal error encountered."}}`, []string{"(500)", "Internal error"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := apiService(t, func(*http.Request) (*http.Response, error) { return jsonResponse(c.status, c.body), nil })
			_, err := s.Spaces(context.Background(), "acct-a", "")
			if err == nil {
				t.Fatal("sem erro")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err = %q, want contendo %q", err, w)
				}
			}
		})
	}
}
