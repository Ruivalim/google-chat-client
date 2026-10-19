package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/ruivalim/google-chat-client/locales"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var spacePattern = regexp.MustCompile(`^spaces/[a-zA-Z0-9_-]+$`)
var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,64}$|^[a-fA-F0-9-]{36}$`)

// send assina a requisição com o token da conta e a envia pelo client dado.
func (s *Service) send(ctx context.Context, client *http.Client, accountID string, req *http.Request) (*http.Response, error) {
	token, err := s.accessToken(ctx, accountID)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := client.Do(req)
	if err != nil {
		return nil, errors.New(locales.ErrNetwork())
	}
	return r, nil
}

// transfer serve para upload e download: sem o timeout total do client da API,
// que cortaria um arquivo grande no meio. Quem limita é o contexto.
func (s *Service) transfer() *http.Client { return &http.Client{Transport: s.http.Transport} }

func (s *Service) api(ctx context.Context, accountID, method, path string, query url.Values, body any, out any) error {
	var err error
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, s.apiURL+"/"+path+"?"+query.Encode(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := s.send(ctx, s.http, accountID, req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return apiError(r)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(out); err != nil {
		return errors.New(locales.ErrBadResponse())
	}
	return nil
}

// googleError é o corpo de erro das APIs do Google: a mensagem é legível e o
// reason do ErrorInfo diz a causa exata, como API desativada no projeto.
type googleError struct {
	Error struct {
		Message string `json:"message"`
		Details []struct {
			Reason string `json:"reason"`
		} `json:"details"`
	} `json:"error"`
}

func apiError(r *http.Response) error {
	var body googleError
	_ = json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body)
	said := strings.TrimSpace(body.Error.Message)
	reasons := []string{}
	for _, d := range body.Error.Details {
		reasons = append(reasons, d.Reason)
	}
	with := func(text string) error {
		if said == "" {
			return errors.New(text)
		}
		return errors.New(locales.ErrGoogleSaid(text, said))
	}
	switch {
	case slices.Contains(reasons, "SERVICE_DISABLED"):
		return with(locales.ErrServiceDisabled())
	case slices.Contains(reasons, "ACCESS_TOKEN_SCOPE_INSUFFICIENT"):
		return with(locales.ErrScopeMissing())
	case r.StatusCode == http.StatusNotFound && strings.Contains(strings.ToLower(said), "chat app not found"):
		return with(locales.ErrChatAppMissing())
	case r.StatusCode == http.StatusUnauthorized:
		return errors.New(locales.ErrUnauthorized())
	case r.StatusCode == http.StatusForbidden:
		return with(locales.ErrForbidden())
	case r.StatusCode == http.StatusTooManyRequests:
		return errors.New(locales.ErrRateLimited())
	case r.StatusCode == http.StatusNotFound:
		return errors.New(locales.ErrNotFound())
	}
	return with(locales.ErrStatus(strconv.Itoa(r.StatusCode)))
}

func (s *Service) Spaces(ctx context.Context, accountID, pageToken string) (SpacePage, error) {
	page := SpacePage{Spaces: []Space{}}
	q := url.Values{"pageSize": {"100"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	if err := s.api(ctx, accountID, http.MethodGet, "spaces", q, nil, &page); err != nil {
		return page, err
	}
	x, err := s.accountSession(accountID)
	if err != nil {
		return page, err
	}
	var wg sync.WaitGroup
	limit := make(chan struct{}, 4)
	for i := range page.Spaces {
		space := &page.Spaces[i]
		if space.DisplayName != "" {
			continue
		}
		if label, ok := x.labels.Load(space.Name); ok {
			space.DisplayName = label.(string)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case limit <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-limit }()
			name, err := s.spaceLabel(ctx, accountID, space.Name)
			if err == nil && name != "" {
				space.DisplayName = name
				x.labels.Store(space.Name, name)
			}
		}()
	}
	wg.Wait()
	slices.SortStableFunc(page.Spaces, func(a, b Space) int { return strings.Compare(b.LastActiveTime, a.LastActiveTime) })
	return page, nil
}

func (s *Service) spaceLabel(ctx context.Context, accountID, space string) (string, error) {
	if !spacePattern.MatchString(space) {
		return "", errors.New(locales.ErrInvalidSpace())
	}
	var page struct {
		Memberships []struct {
			Member User `json:"member"`
		} `json:"memberships"`
	}
	if err := s.api(ctx, accountID, http.MethodGet, space+"/members", url.Values{"pageSize": {"20"}}, nil, &page); err != nil {
		return "", err
	}
	names := []string{}
	for _, m := range page.Memberships {
		if m.Member.Name == "users/"+accountID {
			continue
		}
		// Sem nome nem email, o ID numérico não ajuda ninguém; a interface mostra o nome genérico.
		name := m.Member.DisplayName
		if name == "" {
			name = m.Member.Email
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", "), nil
}

func (s *Service) Messages(ctx context.Context, accountID, space, pageToken, thread string) (MessagePage, error) {
	page := MessagePage{Messages: []Message{}}
	if !spacePattern.MatchString(space) {
		return page, errors.New(locales.ErrInvalidSpace())
	}
	q := url.Values{"pageSize": {"50"}, "orderBy": {"create_time DESC"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	if thread != "" {
		if !validThread(space, thread) {
			return page, errors.New(locales.ErrThreadMismatch())
		}
		q.Set("filter", "thread.name = "+thread)
	}
	if err := s.api(ctx, accountID, http.MethodGet, space+"/messages", q, nil, &page); err != nil {
		return page, err
	}
	slices.Reverse(page.Messages)
	return page, nil
}

// SendMessage envia texto, um anexo já enviado com UploadAttachment, ou os dois.
func (s *Service) SendMessage(ctx context.Context, accountID, space, text, thread, id string, attachment *AttachmentDataRef) (Message, error) {
	if !spacePattern.MatchString(space) {
		return Message{}, errors.New(locales.ErrInvalidSpace())
	}
	if attachment != nil && attachment.AttachmentUploadToken == "" {
		return Message{}, errors.New(locales.ErrAttachmentNotUploaded())
	}
	if (strings.TrimSpace(text) == "" && attachment == nil) || len(text) > 32000 {
		return Message{}, errors.New(locales.ErrMessageSize())
	}
	if id == "" {
		id = requestID()
	}
	if !requestIDPattern.MatchString(id) {
		return Message{}, errors.New(locales.ErrInvalidRequestId())
	}
	q := url.Values{"requestId": {id}}
	body := map[string]any{"text": text}
	if attachment != nil {
		body["attachment"] = []map[string]any{{"attachmentDataRef": attachment}}
	}
	if thread != "" {
		if !validThread(space, thread) {
			return Message{}, errors.New(locales.ErrThreadMismatch())
		}
		body["thread"] = Thread{Name: thread}
		q.Set("messageReplyOption", "REPLY_MESSAGE_OR_FAIL")
	}
	var message Message
	if err := s.api(ctx, accountID, http.MethodPost, space+"/messages", q, body, &message); err != nil {
		return Message{}, err
	}
	return message, nil
}

func validThread(space, thread string) bool {
	return strings.HasPrefix(thread, space+"/threads/") && regexp.MustCompile(`^spaces/[a-zA-Z0-9_-]+/threads/[a-zA-Z0-9_.-]+$`).MatchString(thread)
}
