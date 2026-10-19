package chat

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Activity é uma mensagem nova de outra pessoa, achada pelo Watcher.
type Activity struct {
	AccountID string  `json:"accountId"`
	Space     Space   `json:"space"`
	Message   Message `json:"message"`
}

// Unread conta as mensagens que chegaram numa conversa desde que o app abriu
// e que a pessoa ainda não viu. Não é o estado de leitura do Google, que pede
// outro escopo.
type Unread struct {
	AccountID string `json:"accountId"`
	Space     string `json:"space"`
	Count     int    `json:"count"`
}

// Até quantas notificações uma conversa gera numa rodada; o resto só conta.
const notifyPerSpace = 3

type accountWatch struct {
	since  time.Time            // primeira rodada: o que é mais velho já existia
	spaces map[string]time.Time // lastActiveTime de cada espaço na última rodada
	unread map[string]int
}

// Watcher consulta, no intervalo das configurações, quais conversas tiveram
// atividade em cada conta e avisa das mensagens novas de outras pessoas.
type Watcher struct {
	s       *Service
	notify  func(Activity)
	changed func([]Unread)
	focused func() bool

	mu          sync.Mutex
	known       map[string]*accountWatch
	openAccount string
	openSpace   string
	wake        chan struct{}
}

func NewWatcher(s *Service, notify func(Activity), changed func([]Unread), focused func() bool) *Watcher {
	return &Watcher{s: s, notify: notify, changed: changed, focused: focused, known: map[string]*accountWatch{}, wake: make(chan struct{}, 1)}
}

// Run roda até ctx acabar. Mudar as configurações acorda o laço na hora.
func (w *Watcher) Run(ctx context.Context) {
	for {
		settings := w.s.State().Notifications
		var tick <-chan time.Time
		if settings.On {
			w.check(ctx)
			tick = time.After(time.Duration(settings.Every) * time.Second)
		}
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-tick:
		}
	}
}

// Wake faz a próxima rodada acontecer agora, com as configurações novas.
func (w *Watcher) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// SetOpen informa a conversa na tela, que não gera notificação nem não lida
// enquanto a janela está em foco, e zera as não lidas dela.
func (w *Watcher) SetOpen(accountID, space string) {
	w.mu.Lock()
	w.openAccount, w.openSpace = accountID, space
	cleared := false
	if a := w.known[accountID]; a != nil && a.unread[space] > 0 {
		delete(a.unread, space)
		cleared = true
	}
	snapshot := w.unreadLocked()
	w.mu.Unlock()
	if cleared {
		w.changed(snapshot)
	}
}

func (w *Watcher) Unread() []Unread {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.unreadLocked()
}

func (w *Watcher) unreadLocked() []Unread {
	list := []Unread{}
	for id, a := range w.known {
		for space, n := range a.unread {
			list = append(list, Unread{AccountID: id, Space: space, Count: n})
		}
	}
	return list
}

func (w *Watcher) check(ctx context.Context) {
	accounts := w.s.State().Accounts
	w.mu.Lock()
	dropped := false
	for id, a := range w.known {
		if !containsAccount(accounts, id) {
			dropped = dropped || len(a.unread) > 0
			delete(w.known, id)
		}
	}
	snapshot := w.unreadLocked()
	w.mu.Unlock()
	if dropped {
		w.changed(snapshot)
	}
	for _, a := range accounts {
		if ctx.Err() != nil {
			return
		}
		// Uma conta com problema (sessão vencida, API desativada) não para as outras.
		if err := w.checkAccount(ctx, a.ID); err != nil {
			log.Printf("notificações de %s: %v", a.Email, err)
		}
	}
}

func containsAccount(list []Account, id string) bool {
	for _, a := range list {
		if a.ID == id {
			return true
		}
	}
	return false
}

func (w *Watcher) checkAccount(ctx context.Context, id string) error {
	spaces, err := w.s.activeSpaces(ctx, id)
	if err != nil {
		return err
	}
	w.mu.Lock()
	a := w.known[id]
	if a == nil {
		a = &accountWatch{since: time.Now(), spaces: map[string]time.Time{}, unread: map[string]int{}}
		for _, sp := range spaces {
			a.spaces[sp.Name] = activeTime(sp)
		}
		w.known[id] = a
		w.mu.Unlock()
		return nil
	}
	type pending struct {
		space Space
		after time.Time
	}
	todo := []pending{}
	for _, sp := range spaces {
		prev, ok := a.spaces[sp.Name]
		if !ok {
			prev = a.since
		}
		if activeTime(sp).After(prev) {
			todo = append(todo, pending{sp, prev})
		}
	}
	w.mu.Unlock()

	// Fora do mutex: IsFocused pergunta à janela pela thread principal.
	focused := len(todo) > 0 && w.focused()
	changed := false
	for _, p := range todo {
		messages, err := w.s.messagesAfter(ctx, id, p.space.Name, p.after)
		if err != nil {
			// A linha de base fica onde estava, e a próxima rodada tenta de novo.
			log.Printf("mensagens novas em %s: %v", p.space.Name, err)
			continue
		}
		space := p.space
		if space.DisplayName == "" {
			space.DisplayName = w.s.cachedLabel(ctx, id, space.Name)
		}
		notified := 0
		w.mu.Lock()
		a.spaces[space.Name] = activeTime(p.space)
		for _, m := range messages {
			if m.Sender.Name == "users/"+id {
				continue
			}
			if focused && w.openAccount == id && w.openSpace == space.Name {
				continue
			}
			a.unread[space.Name]++
			changed = true
			if notified < notifyPerSpace {
				notified++
				activity := Activity{AccountID: id, Space: space, Message: m}
				w.mu.Unlock()
				w.notify(activity)
				w.mu.Lock()
			}
		}
		w.mu.Unlock()
	}
	if changed {
		w.changed(w.Unread())
	}
	return nil
}

func activeTime(sp Space) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, sp.LastActiveTime)
	return t
}

// activeSpaces lista os espaços sem resolver nomes: só serve para comparar
// lastActiveTime, e roda a cada intervalo.
func (s *Service) activeSpaces(ctx context.Context, accountID string) ([]Space, error) {
	all := []Space{}
	token := ""
	for range 5 {
		var page SpacePage
		q := url.Values{"pageSize": {"1000"}}
		if token != "" {
			q.Set("pageToken", token)
		}
		if err := s.api(ctx, accountID, http.MethodGet, "spaces", q, nil, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Spaces...)
		if token = page.NextPageToken; token == "" {
			return all, nil
		}
	}
	return all, nil
}

func (s *Service) messagesAfter(ctx context.Context, accountID, space string, after time.Time) ([]Message, error) {
	if !spacePattern.MatchString(space) {
		return nil, errors.New("identificador de conversa inválido")
	}
	q := url.Values{
		"pageSize": {"25"},
		"orderBy":  {"create_time ASC"},
		"filter":   {fmt.Sprintf("createTime > %q", after.UTC().Format(time.RFC3339Nano))},
	}
	var page MessagePage
	if err := s.api(ctx, accountID, http.MethodGet, space+"/messages", q, nil, &page); err != nil {
		return nil, err
	}
	return page.Messages, nil
}

// cachedLabel dá nome a uma DM ou grupo pelos membros, com o mesmo cache da
// lista de conversas.
func (s *Service) cachedLabel(ctx context.Context, accountID, space string) string {
	x, err := s.accountSession(accountID)
	if err != nil {
		return ""
	}
	if label, ok := x.labels.Load(space); ok {
		return label.(string)
	}
	name, err := s.spaceLabel(ctx, accountID, space)
	if err != nil || strings.TrimSpace(name) == "" {
		return ""
	}
	x.labels.Store(space, name)
	return name
}
