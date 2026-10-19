package chat

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

var scopes = []string{
	"openid", "email", "profile",
	"https://www.googleapis.com/auth/chat.spaces.readonly",
	"https://www.googleapis.com/auth/chat.memberships.readonly",
	"https://www.googleapis.com/auth/chat.messages",
}

func oauthConfig(c Credentials, redirect string) *oauth2.Config {
	return &oauth2.Config{
		ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: redirect,
		Scopes: scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL: "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

func (s *Service) AddAccount(parent context.Context, openBrowser func(string) error) (Account, error) {
	if !s.authMu.TryLock() {
		return Account{}, errors.New("já existe um login em andamento; conclua ou cancele antes de iniciar outro")
	}
	defer s.authMu.Unlock()
	c, err := s.credentials()
	if err != nil {
		return Account{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return Account{}, fmt.Errorf("abrir o retorno local do login: %w", err)
	}
	defer listener.Close()
	state := rand.Text()
	verifier := oauth2.GenerateVerifier()
	cfg := oauthConfig(c, "http://"+listener.Addr().String()+"/oauth/callback")
	type callback struct { code string; err error }
	result := make(chan callback, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state)) != 1 {
			http.Error(w, "Retorno de login inválido. Volte ao aplicativo e tente novamente.", http.StatusBadRequest)
			return
		}
		value := callback{code: r.URL.Query().Get("code")}
		if r.URL.Query().Get("error") != "" || value.code == "" {
			value.err = errors.New("o Google não autorizou o acesso; confira o consentimento e as políticas do Workspace")
		}
		select {
		case result <- value:
			fmt.Fprint(w, "Pode voltar ao Google Chat Client. O aplicativo está concluindo a conexão da conta.")
		default:
			http.Error(w, "Este retorno já foi recebido.", http.StatusGone)
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5*time.Second, MaxHeaderBytes: 8192}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "consent select_account"))
	if err := openBrowser(url); err != nil {
		return Account{}, errors.New("não foi possível abrir o navegador do sistema")
	}
	var response callback
	select {
	case response = <-result:
	case <-ctx.Done():
		return Account{}, errors.New("o login foi cancelado ou expirou; tente conectar a conta novamente")
	}
	if response.err != nil {
		return Account{}, response.err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, s.http)
	token, err := cfg.Exchange(ctx, response.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Account{}, errors.New("o Google não concluiu o login; confira se o cliente OAuth é do tipo Aplicativo para computador")
	}
	if granted, ok := token.Extra("scope").(string); ok {
		for _, required := range scopes[3:] {
			if !slices.Contains(strings.Fields(granted), required) {
				return Account{}, errors.New("faltou autorizar uma permissão de Chat; conecte novamente e aceite as permissões solicitadas")
			}
		}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	r, err := s.http.Do(req)
	if err != nil {
		return Account{}, errors.New("não foi possível consultar a identidade da conta Google")
	}
	defer r.Body.Close()
	var user struct {
		Sub string `json:"sub"`
		Name string `json:"name"`
		Email string `json:"email"`
		Verified bool `json:"email_verified"`
		Domain string `json:"hd"`
	}
	if r.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&user) != nil || !user.Verified || !accountIDPattern.MatchString(user.Sub) || user.Email == "" {
		return Account{}, errors.New("o Google não retornou uma identidade de conta válida")
	}
	account := Account{ID: user.Sub, Name: user.Name, Email: user.Email, Domain: user.Domain}
	if err := s.attachAccount(account, token); err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Service) attachAccount(a Account, token *oauth2.Token) error {
	if token.RefreshToken == "" {
		return errors.New("o Google não forneceu uma sessão persistente; tente conectar novamente aceitando o consentimento")
	}
	s.mu.RLock()
	x := s.sessions[a.ID]
	s.mu.RUnlock()
	if x == nil { x = &session{} }
	x.mu.Lock()
	defer x.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.vault.Set("refresh:"+a.ID, []byte(token.RefreshToken)); err != nil {
		return fmt.Errorf("guardar sessão no chaveiro do sistema: %w", err)
	}
	next := s.config
	next.Accounts = slices.Clone(s.config.Accounts)
	index := slices.IndexFunc(next.Accounts, func(v Account) bool { return v.ID == a.ID })
	if index >= 0 { next.Accounts[index] = a } else { next.Accounts = append(next.Accounts, a) }
	next.ActiveAccountID = a.ID
	if err := s.saveLocked(next); err != nil { return err }
	x.token, x.removed = token, false
	s.sessions[a.ID] = x
	return nil
}

func requestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8]+"-"+h[8:12]+"-"+h[12:16]+"-"+h[16:20]+"-"+h[20:]
}
