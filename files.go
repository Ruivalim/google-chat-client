package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/ruivalim/google-chat-client/internal/chat"
)

// PickedFile é um arquivo escolhido no diálogo nativo. A interface só conhece o
// ID: o caminho fica no Go, e o upload aceita apenas arquivos escolhidos assim.
type PickedFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	TooLarge bool   `json:"tooLarge"`
}

func (d *Desktop) PickFiles(ctx context.Context) ([]PickedFile, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: mygo.CallerWindow(ctx), Title: "Anexar arquivos", Multiple: true,
	})
	if err != nil {
		return nil, err
	}
	picked := []PickedFile{}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.picked == nil {
		d.picked = map[string]string{}
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		var b [12]byte
		_, _ = rand.Read(b[:])
		id := hex.EncodeToString(b[:])
		d.picked[id] = p
		picked = append(picked, PickedFile{ID: id, Name: info.Name(), Size: info.Size(), TooLarge: info.Size() > chat.MaxAttachmentSize})
	}
	return picked, nil
}

// ForgetFile solta um arquivo escolhido que a pessoa tirou da mensagem ou que
// já foi enviado.
func (d *Desktop) ForgetFile(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.picked, id)
}

func (d *Desktop) UploadAttachment(ctx context.Context, accountID, space, fileID string) (chat.AttachmentDataRef, error) {
	d.mu.Lock()
	path, ok := d.picked[fileID]
	d.mu.Unlock()
	if !ok {
		return chat.AttachmentDataRef{}, errors.New("escolha o arquivo de novo")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	return d.service.UploadAttachment(ctx, accountID, space, path)
}

// DownloadAttachment pergunta onde salvar e grava o anexo ali. Devolve o
// caminho salvo, ou "" se a pessoa cancelou.
func (d *Desktop) DownloadAttachment(ctx context.Context, accountID, resourceName, filename string) (string, error) {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "." || name == "/" || name == "" {
		name = "anexo"
	}
	home, _ := mygo.App.Path(mygo.PathHome)
	dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: mygo.CallerWindow(ctx), Title: "Salvar anexo", DefaultPath: filepath.Join(home, "Downloads", name),
	})
	if err != nil || dest == "" {
		return "", err
	}
	// Grava ao lado e renomeia no fim: um download interrompido não deixa
	// arquivo pela metade com o nome final.
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	_, err = d.service.DownloadAttachment(ctx, accountID, resourceName, tmp)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return "", err
	}
	return dest, nil
}

func (d *Desktop) ShowInFolder(path string) {
	if filepath.IsAbs(path) {
		mygo.Shell.ShowItemInFolder(path)
	}
}
