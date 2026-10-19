package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUploadAttachmentSendsMultipartRelated(t *testing.T) {
	content := bytes.Repeat([]byte("relatório trimestral\n"), 500)
	path := writeFile(t, "relatório final.pdf", content)
	var got struct {
		path, query, auth string
		meta              map[string]string
		partType          string
		data              []byte
		declared, real    int64
	}
	s := apiService(t, func(r *http.Request) (*http.Response, error) {
		got.path, got.query, got.auth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		got.declared, got.real = r.ContentLength, int64(len(body))
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/related" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
			return jsonResponse(http.StatusBadRequest, `{}`), nil
		}
		mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		first, err := mr.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewDecoder(first).Decode(&got.meta)
		second, err := mr.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		got.partType = second.Header.Get("Content-Type")
		got.data, _ = io.ReadAll(second)
		return jsonResponse(http.StatusOK, `{"attachmentDataRef":{"resourceName":"r/1","attachmentUploadToken":"tok-1"}}`), nil
	})

	ref, err := s.UploadAttachment(context.Background(), "acct-a", "spaces/s1", path)
	if err != nil {
		t.Fatal(err)
	}
	if ref.AttachmentUploadToken != "tok-1" || ref.ResourceName != "r/1" {
		t.Fatalf("ref = %+v", ref)
	}
	if got.path != "/upload/v1/spaces/s1/attachments:upload" || got.query != "uploadType=multipart" || got.auth != "Bearer token" {
		t.Fatalf("pedido = %s?%s (%s)", got.path, got.query, got.auth)
	}
	if got.meta["filename"] != "relatório final.pdf" || got.partType != "application/pdf" || !bytes.Equal(got.data, content) {
		t.Fatalf("partes: meta=%v tipo=%q bytes iguais=%v", got.meta, got.partType, bytes.Equal(got.data, content))
	}
	if got.declared != got.real {
		t.Fatalf("Content-Length %d, corpo de %d bytes", got.declared, got.real)
	}
}

func TestUploadAttachmentSniffsTypeWithoutExtension(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	path := writeFile(t, "captura", png)
	var partType string
	var data []byte
	s := apiService(t, func(r *http.Request) (*http.Response, error) {
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		_, _ = mr.NextPart()
		p, _ := mr.NextPart()
		partType = p.Header.Get("Content-Type")
		data, _ = io.ReadAll(p)
		return jsonResponse(http.StatusOK, `{"attachmentDataRef":{"attachmentUploadToken":"t"}}`), nil
	})
	if _, err := s.UploadAttachment(context.Background(), "acct-a", "spaces/s1", path); err != nil {
		t.Fatal(err)
	}
	if partType != "image/png" || !bytes.Equal(data, png) {
		t.Fatalf("tipo = %q; o arquivo foi enviado a partir do começo: %v", partType, bytes.Equal(data, png))
	}
}

func TestUploadAttachmentRejectsBeforeSending(t *testing.T) {
	called := false
	s := apiService(t, func(*http.Request) (*http.Response, error) {
		called = true
		return jsonResponse(http.StatusOK, `{}`), nil
	})
	big := filepath.Join(t.TempDir(), "grande.iso")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxAttachmentSize + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	ok := writeFile(t, "ok.txt", []byte("oi"))
	cases := map[string][2]string{
		"maior que 200 MB": {"spaces/s1", big},
		"pasta":            {"spaces/s1", t.TempDir()},
		"não existe":       {"spaces/s1", filepath.Join(t.TempDir(), "sumiu.txt")},
		"espaço inválido":  {"spaces/s1/../outro", ok},
	}
	for name, c := range cases {
		if _, err := s.UploadAttachment(context.Background(), "acct-a", c[0], c[1]); err == nil {
			t.Errorf("%s: aceito", name)
		}
	}
	if called {
		t.Fatal("arquivo recusado chegou ao Google")
	}
}

func TestUploadAttachmentReportsGoogleError(t *testing.T) {
	path := writeFile(t, "a.txt", []byte("oi"))
	s := apiService(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusForbidden, `{"error":{"message":"Attachments are disabled by the administrator."}}`), nil
	})
	_, err := s.UploadAttachment(context.Background(), "acct-a", "spaces/s1", path)
	if err == nil || !strings.Contains(err.Error(), "disabled by the administrator") {
		t.Fatalf("err = %v", err)
	}
	s = apiService(t, func(*http.Request) (*http.Response, error) { return jsonResponse(http.StatusOK, `{}`), nil })
	if _, err := s.UploadAttachment(context.Background(), "acct-a", "spaces/s1", path); err == nil {
		t.Fatal("resposta sem token aceita como upload concluído")
	}
}

func TestDownloadAttachment(t *testing.T) {
	var gotPath, gotQuery string
	s := apiService(t, func(r *http.Request) (*http.Response, error) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
		return jsonResponse(http.StatusOK, "conteúdo do anexo"), nil
	})
	var buf bytes.Buffer
	n, err := s.DownloadAttachment(context.Background(), "acct-a", "spaces/s1/attachments/ab c=", &buf)
	if err == nil {
		t.Fatal("aceitou nome com espaço")
	}
	n, err = s.DownloadAttachment(context.Background(), "acct-a", "spaces/s1/attachments/abc=", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "conteúdo do anexo" || n != int64(buf.Len()) {
		t.Fatalf("conteúdo = %q (%d)", buf.String(), n)
	}
	if gotPath != "/v1/media/spaces/s1/attachments/abc=" || gotQuery != "alt=media" {
		t.Fatalf("pedido = %s?%s", gotPath, gotQuery)
	}

	called := false
	s = apiService(t, func(*http.Request) (*http.Response, error) {
		called = true
		return jsonResponse(http.StatusOK, ""), nil
	})
	for _, bad := range []string{"", "../spaces/x", "a/../b", "a?alt=json", "a#b", "a//b", strings.Repeat("a", 3000)} {
		if _, err := s.DownloadAttachment(context.Background(), "acct-a", bad, io.Discard); err == nil {
			t.Errorf("aceitou %q", bad)
		}
	}
	if called {
		t.Fatal("nome inválido chegou ao Google")
	}

	s = apiService(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusNotFound, `{"error":{"message":"Requested entity was not found."}}`), nil
	})
	if _, err := s.DownloadAttachment(context.Background(), "acct-a", "x", io.Discard); err == nil {
		t.Fatal("404 virou download")
	}
}

func TestSendMessageWithAttachment(t *testing.T) {
	var body map[string]any
	s := apiService(t, func(r *http.Request) (*http.Response, error) {
		body = map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		return jsonResponse(http.StatusOK, `{"name":"spaces/s1/messages/m"}`), nil
	})
	ref := &AttachmentDataRef{AttachmentUploadToken: "tok-1", ResourceName: "r/1"}
	if _, err := s.SendMessage(context.Background(), "acct-a", "spaces/s1", "", "", "", ref); err != nil {
		t.Fatalf("anexo sem texto: %v", err)
	}
	data, _ := json.Marshal(body["attachment"])
	var sent []struct {
		AttachmentDataRef AttachmentDataRef `json:"attachmentDataRef"`
	}
	if err := json.Unmarshal(data, &sent); err != nil || len(sent) != 1 || sent[0].AttachmentDataRef != *ref {
		t.Fatalf("attachment = %s", data)
	}
	if _, err := s.SendMessage(context.Background(), "acct-a", "spaces/s1", "", "", "", &AttachmentDataRef{ResourceName: "r/1"}); err == nil {
		t.Fatal("anexo sem token aceito")
	}
}
