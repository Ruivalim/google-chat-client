package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

var (
	ErrNotFound      = errors.New("credencial não encontrada")
	accountIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
)

type session struct {
	mu      sync.Mutex
	token   *oauth2.Token
	removed bool
	labels  sync.Map
}

type Service struct {
	mu       sync.RWMutex
	authMu   sync.Mutex
	path     string
	vault    Vault
	config   config
	sessions map[string]*session
	http     *http.Client
	apiURL   string
}

func Open(path string, vault Vault) (*Service, error) {
	s := &Service{
		path: path, vault: vault, sessions: make(map[string]*session),
		http:   &http.Client{Timeout: 25 * time.Second},
		apiURL: "https://chat.googleapis.com/v1",
		config: config{Accounts: []Account{}},
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("ler configuração: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.config); err != nil {
			return nil, fmt.Errorf("a configuração local não pôde ser lida: %w", err)
		}
	}
	if s.config.Accounts == nil {
		s.config.Accounts = []Account{}
	}
	for _, a := range s.config.Accounts {
		if !accountIDPattern.MatchString(a.ID) {
			return nil, errors.New("a configuração contém uma conta inválida")
		}
		s.sessions[a.ID] = &session{}
	}
	return s, nil
}

func (s *Service) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return State{
		Configured: s.config.ClientID != "", ClientID: s.config.ClientID,
		Accounts:        append([]Account{}, s.config.Accounts...),
		ActiveAccountID: s.config.ActiveAccountID,
	}
}

func (s *Service) saveLocked(next config) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".accounts-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	s.config = next
	return nil
}

func (s *Service) Configure(c Credentials) error {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	c.ClientID = strings.TrimSpace(c.ClientID)
	c.ClientSecret = strings.TrimSpace(c.ClientSecret)
	if !strings.HasSuffix(c.ClientID, ".apps.googleusercontent.com") || strings.ContainsAny(c.ClientID, " \r\n\t") {
		return errors.New("importe as credenciais de um cliente OAuth do tipo Aplicativo para computador")
	}
	if c.ClientSecret == "" {
		return errors.New("o arquivo de credenciais não contém client_secret")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.config.Accounts) > 0 && s.config.ClientID != c.ClientID {
		return errors.New("remova as contas conectadas antes de trocar o projeto OAuth")
	}
	data, _ := json.Marshal(c)
	if err := s.vault.Set("oauth-client", data); err != nil {
		return fmt.Errorf("guardar credenciais no chaveiro do sistema: %w", err)
	}
	next := s.config
	next.ClientID = c.ClientID
	return s.saveLocked(next)
}

func (s *Service) SelectAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return errors.New("essa conta não está conectada")
	}
	next := s.config
	next.ActiveAccountID = id
	return s.saveLocked(next)
}

func (s *Service) accountSession(id string) (*session, error) {
	s.mu.RLock()
	x, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.New("essa conta não está conectada")
	}
	return x, nil
}

func (s *Service) RemoveAccount(id string) error {
	x, err := s.accountSession(id)
	if err != nil {
		return err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.config
	next.Accounts = slices.DeleteFunc(slices.Clone(s.config.Accounts), func(a Account) bool { return a.ID == id })
	if next.ActiveAccountID == id {
		next.ActiveAccountID = ""
		if len(next.Accounts) > 0 {
			next.ActiveAccountID = next.Accounts[0].ID
		}
	}
	if err := s.vault.Delete("refresh:" + id); err != nil {
		return fmt.Errorf("remover credencial do chaveiro: %w", err)
	}
	if err := s.saveLocked(next); err != nil {
		x.token = nil
		return err
	}
	x.removed, x.token = true, nil
	delete(s.sessions, id)
	return nil
}

func (s *Service) credentials() (Credentials, error) {
	data, err := s.vault.Get("oauth-client")
	if err != nil {
		return Credentials{}, errors.New("importe as credenciais OAuth nas configurações; confira se o chaveiro do sistema está desbloqueado")
	}
	var c Credentials
	if json.Unmarshal(data, &c) != nil || c.ClientID == "" || c.ClientSecret == "" {
		return c, errors.New("as credenciais OAuth são inválidas; importe o arquivo novamente")
	}
	return c, nil
}

func (s *Service) accessToken(ctx context.Context, id string) (string, error) {
	x, err := s.accountSession(id)
	if err != nil {
		return "", err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.removed {
		return "", errors.New("essa conta foi desconectada")
	}
	if x.token != nil && x.token.Valid() {
		return x.token.AccessToken, nil
	}
	if x.token == nil {
		refresh, err := s.vault.Get("refresh:" + id)
		if err != nil {
			return "", errors.New("não foi possível recuperar a sessão; desbloqueie o chaveiro ou conecte essa conta novamente")
		}
		x.token = &oauth2.Token{RefreshToken: string(refresh)}
	}
	c, err := s.credentials()
	if err != nil {
		return "", err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, s.http)
	token, err := oauthConfig(c, "").TokenSource(ctx, x.token).Token()
	if err != nil {
		// invalid_grant é sessão revogada ou vencida, por exemplo os 7 dias de um
		// projeto em modo de teste. Tentar de novo não resolve.
		var refused *oauth2.RetrieveError
		if errors.As(err, &refused) && refused.ErrorCode == "invalid_grant" {
			return "", errors.New("a sessão desta conta expirou ou foi revogada; conecte a conta de novo")
		}
		return "", errors.New("não foi possível renovar a sessão; confira a conexão e, se persistir, conecte essa conta novamente")
	}
	if token.RefreshToken != "" && token.RefreshToken != x.token.RefreshToken {
		if err := s.vault.Set("refresh:"+id, []byte(token.RefreshToken)); err != nil {
			return "", fmt.Errorf("guardar a sessão renovada: %w", err)
		}
	}
	x.token = token
	return token.AccessToken, nil
}
