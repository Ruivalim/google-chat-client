package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
)

var spacePattern = regexp.MustCompile(`^spaces/[a-zA-Z0-9_-]+$`)
var requestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,64}$|^[a-fA-F0-9-]{36}$`)

func (s *Service) api(ctx context.Context, accountID, method, path string, query url.Values, body any, out any) error {
	token, err := s.accessToken(ctx, accountID)
	if err != nil {
		return err
	}
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
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	r, err := s.http.Do(req)
	if err != nil {
		return errors.New("não foi possível alcançar o Google Chat; confira a conexão e tente novamente")
	}
	defer r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return apiError(r)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(out); err != nil {
		return errors.New("a resposta do Google Chat não pôde ser lida")
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
		return fmt.Errorf("%s. O Google disse: %s", text, said)
	}
	switch {
	case slices.Contains(reasons, "SERVICE_DISABLED"):
		return with("a Google Chat API não está ativada no projeto desta conta; ative em APIs e serviços > Biblioteca, no Google Cloud")
	case slices.Contains(reasons, "ACCESS_TOKEN_SCOPE_INSUFFICIENT"):
		return with("faltou uma permissão de Chat nesta sessão; confira os escopos em Acesso a dados e conecte a conta de novo")
	case r.StatusCode == http.StatusNotFound && strings.Contains(strings.ToLower(said), "chat app not found"):
		return with("o app de Chat não está configurado no projeto; preencha nome, ícone e descrição na aba Configuração da Google Chat API")
	case r.StatusCode == http.StatusUnauthorized:
		return errors.New("a sessão foi recusada; conecte essa conta novamente")
	case r.StatusCode == http.StatusForbidden:
		return with("o Google recusou a permissão")
	case r.StatusCode == http.StatusTooManyRequests:
		return errors.New("o Google limitou as consultas; aguarde um pouco antes de atualizar")
	case r.StatusCode == http.StatusNotFound:
		return errors.New("essa conversa ou mensagem não está mais disponível para a conta selecionada")
	}
	return with(fmt.Sprintf("o Google Chat retornou um erro (%d)", r.StatusCode))
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
		return "", errors.New("identificador de conversa inválido")
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
		return page, errors.New("identificador de conversa inválido")
	}
	q := url.Values{"pageSize": {"50"}, "orderBy": {"create_time DESC"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	if thread != "" {
		if !validThread(space, thread) {
			return page, errors.New("a thread não pertence à conversa selecionada")
		}
		q.Set("filter", "thread.name = "+thread)
	}
	if err := s.api(ctx, accountID, http.MethodGet, space+"/messages", q, nil, &page); err != nil {
		return page, err
	}
	slices.Reverse(page.Messages)
	return page, nil
}

func (s *Service) SendMessage(ctx context.Context, accountID, space, text, thread, id string) (Message, error) {
	if !spacePattern.MatchString(space) {
		return Message{}, errors.New("identificador de conversa inválido")
	}
	if strings.TrimSpace(text) == "" || len(text) > 32000 {
		return Message{}, errors.New("escreva uma mensagem de até 32.000 bytes")
	}
	if id == "" {
		id = requestID()
	}
	if !requestIDPattern.MatchString(id) {
		return Message{}, errors.New("identificador de envio inválido")
	}
	q := url.Values{"requestId": {id}}
	body := map[string]any{"text": text}
	if thread != "" {
		if !validThread(space, thread) {
			return Message{}, errors.New("a thread não pertence à conversa selecionada")
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
