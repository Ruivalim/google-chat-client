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
func (systemVault) Delete(key string) error            { return mygo.Secrets.Delete(key) }

type Desktop struct {
	service     *chat.Service
	mu          sync.Mutex
	loginCancel context.CancelFunc
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
	err = d.service.Configure(file.Installed)
	return d.State(), err
}

func (d *Desktop) ConnectAccount(ctx context.Context) (chat.State, error) {
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
		d.loginCancel = nil
		d.mu.Unlock()
	}()
	_, err := d.service.AddAccount(ctx, mygo.Shell.OpenExternal)
	return d.State(), err
}

func (d *Desktop) CancelLogin() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.loginCancel != nil {
		d.loginCancel()
	}
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
