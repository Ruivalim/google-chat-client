package chat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// MaxAttachmentSize é o limite do upload da API do Chat.
const MaxAttachmentSize = 200 << 20

// UploadAttachment sobe um arquivo local para o espaço e devolve a referência
// que a mensagem leva. O arquivo vai em streaming, sem carregar na memória.
func (s *Service) UploadAttachment(ctx context.Context, accountID, space, path string) (AttachmentDataRef, error) {
	if !spacePattern.MatchString(space) {
		return AttachmentDataRef{}, errors.New("identificador de conversa inválido")
	}
	f, err := os.Open(path)
	if err != nil {
		return AttachmentDataRef{}, fmt.Errorf("abrir o arquivo: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return AttachmentDataRef{}, errors.New("escolha um arquivo comum, não uma pasta")
	}
	if info.Size() > MaxAttachmentSize {
		return AttachmentDataRef{}, fmt.Errorf("%s tem mais de 200 MB, o limite do Google Chat", info.Name())
	}
	contentType, err := sniff(f, info.Name())
	if err != nil {
		return AttachmentDataRef{}, err
	}

	var b [12]byte
	_, _ = rand.Read(b[:])
	boundary := "gcc" + hex.EncodeToString(b[:])
	meta, _ := json.Marshal(map[string]string{"filename": info.Name()})
	head := fmt.Sprintf("--%s\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n%s\r\n--%s\r\nContent-Type: %s\r\n\r\n", boundary, meta, boundary, contentType)
	tail := fmt.Sprintf("\r\n--%s--\r\n", boundary)
	body := io.MultiReader(strings.NewReader(head), f, strings.NewReader(tail))

	endpoint := strings.Replace(s.apiURL, "/v1", "/upload/v1", 1) + "/" + space + "/attachments:upload?uploadType=multipart"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return AttachmentDataRef{}, err
	}
	req.ContentLength = int64(len(head)) + info.Size() + int64(len(tail))
	req.Header.Set("Content-Type", "multipart/related; boundary="+boundary)
	r, err := s.send(ctx, s.transfer(), accountID, req)
	if err != nil {
		return AttachmentDataRef{}, err
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return AttachmentDataRef{}, apiError(r)
	}
	var out struct {
		AttachmentDataRef AttachmentDataRef `json:"attachmentDataRef"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&out); err != nil || out.AttachmentDataRef.AttachmentUploadToken == "" {
		return AttachmentDataRef{}, errors.New("o Google não confirmou o envio do arquivo")
	}
	return out.AttachmentDataRef, nil
}

// sniff escolhe o tipo pela extensão e, sem ela, pelos primeiros bytes.
func sniff(f *os.File, name string) (string, error) {
	if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); t != "" {
		return t, nil
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("ler o arquivo: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("ler o arquivo: %w", err)
	}
	return http.DetectContentType(bytes.Clone(head[:n])), nil
}

// DownloadAttachment grava o conteúdo de um anexo em w e devolve quantos bytes
// vieram. resourceName vem de Attachment.AttachmentDataRef.
func (s *Service) DownloadAttachment(ctx context.Context, accountID, resourceName string, w io.Writer) (int64, error) {
	if !validResourceName(resourceName) {
		return 0, errors.New("referência de anexo inválida")
	}
	segments := strings.Split(resourceName, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL+"/media/"+strings.Join(segments, "/")+"?alt=media", nil)
	if err != nil {
		return 0, err
	}
	r, err := s.send(ctx, s.transfer(), accountID, req)
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return 0, apiError(r)
	}
	n, err := io.Copy(w, io.LimitReader(r.Body, MaxAttachmentSize+1))
	if err != nil {
		return n, errors.New("o download foi interrompido; tente de novo")
	}
	if n > MaxAttachmentSize {
		return n, errors.New("o anexo veio maior que o limite do Google Chat")
	}
	return n, nil
}

// O nome é opaco para quem chama, mas vai no caminho da URL: nada de pedaços
// que mudem de recurso ou de consulta.
func validResourceName(name string) bool {
	if name == "" || len(name) > 2048 || strings.ContainsAny(name, "?#\\ \t\r\n") {
		return false
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}
