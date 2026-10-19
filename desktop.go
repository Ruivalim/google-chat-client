package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/ruivalim/google-chat-client/internal/chat"
)

type systemVault struct{}

func (systemVault) Get(key string) ([]byte, error) {
	data, err := mygo.Secrets.Get(key)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return nil, chat.ErrNotFound
	}
	return data, err
}
func (systemVault) Set(key string, value []byte) error { return mygo.Secrets.Set(key, value) }
func (systemVault) Delete(key string) error {
	if err := mygo.Secrets.Delete(key); err != nil && !errors.Is(err, mygo.ErrSecretNotFound) {
		return err
	}
	return nil
}

type Desktop struct {
	service     *chat.Service
	mu          sync.Mutex
	loginCancel context.CancelFunc
	watcher     *chat.Watcher
	// loginURL é o endereço do login em andamento, para copiar e abrir em outro
	// navegador ou container. Vazio fora de um login.
	loginURL string
}

func (d *Desktop) State() chat.State { return d.service.State() }

func (d *Desktop) ImportCredentials(ctx context.Context) (chat.State, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: mygo.CallerWindow(ctx), Title: "Importar credenciais OAuth para computador",
		Filters: []mygo.FileFilter{{Name: "Credenciais Google (.json)", Extensions: []string{"json"}}},
	})
	if err != nil {
		return d.State(), err
	}
	if len(paths) == 0 {
		return d.State(), nil
	}
	f, err := os.Open(paths[0])
	if err != nil {
		return d.State(), err
	}
	defer f.Close()
	var file struct {
		Installed chat.Credentials `json:"installed"`
	}
	if json.NewDecoder(io.LimitReader(f, 64<<10)).Decode(&file) != nil || file.Installed.ClientID == "" {
		return d.State(), errors.New("selecione o JSON de um cliente OAuth do tipo Aplicativo para computador, baixado no Google Cloud")
	}
	err = d.service.ImportProject(file.Installed)
	return d.State(), err
}

func (d *Desktop) ConnectAccount(ctx context.Context, clientID string) (chat.State, error) {
	d.mu.Lock()
	if d.loginCancel != nil {
		d.mu.Unlock()
		return d.State(), errors.New("já existe um login em andamento")
	}
	ctx, cancel := context.WithCancel(ctx)
	d.loginCancel = cancel
	d.mu.Unlock()
	defer func() {
		cancel()
		d.mu.Lock()
		d.loginCancel, d.loginURL = nil, ""
		d.mu.Unlock()
	}()
	_, err := d.service.AddAccount(ctx, clientID, func(u string) error {
		d.mu.Lock()
		d.loginURL = u
		d.mu.Unlock()
		// Sem navegador padrão o login segue: o link pode ser copiado na janela.
		_ = mygo.Shell.OpenExternal(u)
		return nil
	})
	return d.State(), err
}

func (d *Desktop) LoginURL() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.loginURL
}

// CopyText põe texto curto na área de transferência, como a lista de escopos
// do passo a passo.
func (d *Desktop) CopyText(text string) error {
	if text == "" || len(text) > 4096 {
		return errors.New("texto vazio ou grande demais para copiar")
	}
	return mygo.Clipboard.Write(transfer.TextData(text))
}

func (d *Desktop) CopyLoginURL() error {
	u := d.LoginURL()
	if u == "" {
		return errors.New("não há login em andamento")
	}
	return mygo.Clipboard.Write(transfer.TextData(u))
}

func (d *Desktop) CancelLogin() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.loginCancel != nil {
		d.loginCancel()
	}
}

func (d *Desktop) RemoveProject(clientID string) (chat.State, error) {
	err := d.service.RemoveProject(clientID)
	return d.State(), err
}

func (d *Desktop) SelectAccount(accountID string) (chat.State, error) {
	err := d.service.SelectAccount(accountID)
	return d.State(), err
}

func (d *Desktop) RemoveAccount(accountID string) (chat.State, error) {
	err := d.service.RemoveAccount(accountID)
	return d.State(), err
}

func (d *Desktop) Spaces(ctx context.Context, accountID, pageToken string) (chat.SpacePage, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	return d.service.Spaces(ctx, accountID, pageToken)
}

func (d *Desktop) Messages(ctx context.Context, accountID, space, pageToken, thread string) (chat.MessagePage, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return d.service.Messages(ctx, accountID, space, pageToken, thread)
}

func (d *Desktop) SendMessage(ctx context.Context, accountID, space, text, thread, requestID string) (chat.Message, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return d.service.SendMessage(ctx, accountID, space, text, thread, requestID)
}

func (d *Desktop) OpenLink(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return errors.New("este link não é um endereço web válido")
	}
	return mygo.Shell.OpenExternal(u.String())
}
