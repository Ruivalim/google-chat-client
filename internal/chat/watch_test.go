package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// fakeChat é um Google Chat em memória: espaços com lastActiveTime e mensagens
// que respeitam o filtro createTime > "...".
type fakeChat struct {
	mu        sync.Mutex
	spaces    map[string]Space
	messages  map[string][]Message
	failSpace string
	requests  []string
}

func newFakeChat() *fakeChat {
	return &fakeChat{spaces: map[string]Space{}, messages: map[string][]Message{}}
}

func (f *fakeChat) post(space, name, sender, text string, at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sp := f.spaces[space]
	sp.Name = space
	if sp.SpaceType == "" {
		sp.SpaceType = "DIRECT_MESSAGE"
	}
	sp.LastActiveTime = at.UTC().Format(time.RFC3339Nano)
	f.spaces[space] = sp
	f.messages[space] = append(f.messages[space], Message{
		Name: space + "/messages/" + name, Text: text, CreateTime: sp.LastActiveTime,
		Sender: User{Name: "users/" + sender, DisplayName: map[string]string{"bruno": "Bruno Lima"}[sender]},
	})
}

func (f *fakeChat) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.URL.Path+"?"+r.URL.RawQuery)
	path := strings.TrimPrefix(r.URL.Path, "/v1/")
	switch {
	case path == "spaces":
		page := SpacePage{Spaces: []Space{}}
		for _, sp := range f.spaces {
			page.Spaces = append(page.Spaces, sp)
		}
		data, _ := json.Marshal(page)
		return jsonResponse(http.StatusOK, string(data)), nil
	case strings.HasSuffix(path, "/members"):
		return jsonResponse(http.StatusOK, `{"memberships":[{"member":{"name":"users/acct-a"}},{"member":{"name":"users/bruno","displayName":"Bruno Lima"}}]}`), nil
	case strings.HasSuffix(path, "/messages"):
		space := strings.TrimSuffix(path, "/messages")
		if space == f.failSpace {
			return jsonResponse(http.StatusInternalServerError, `{}`), nil
		}
		filter := r.URL.Query().Get("filter")
		after, err := time.Parse(time.RFC3339Nano, strings.Trim(strings.TrimPrefix(filter, "createTime > "), `"`))
		if err != nil {
			return jsonResponse(http.StatusBadRequest, `{"error":{"message":"filtro inválido"}}`), nil
		}
		page := MessagePage{Messages: []Message{}}
		for _, m := range f.messages[space] {
			if t, _ := time.Parse(time.RFC3339Nano, m.CreateTime); t.After(after) {
				page.Messages = append(page.Messages, m)
			}
		}
		data, _ := json.Marshal(page)
		return jsonResponse(http.StatusOK, string(data)), nil
	}
	return jsonResponse(http.StatusNotFound, `{}`), nil
}

type watchProbe struct {
	mu       sync.Mutex
	notified []Activity
	unread   []Unread
	changes  int
	focused  bool
}

func (p *watchProbe) texts() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []string{}
	for _, a := range p.notified {
		out = append(out, a.Message.Text)
	}
	return out
}

func newWatchTest(t *testing.T) (*Watcher, *fakeChat, *watchProbe) {
	t.Helper()
	chat := newFakeChat()
	s := testService(&memoryVault{values: map[string][]byte{}}, chat)
	s.path = t.TempDir() + "/accounts.json"
	connect(s, "acct-a", testClientID, &session{token: &oauth2.Token{AccessToken: "t", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}})
	probe := &watchProbe{}
	w := NewWatcher(s,
		func(a Activity) { probe.mu.Lock(); probe.notified = append(probe.notified, a); probe.mu.Unlock() },
		func(u []Unread) { probe.mu.Lock(); probe.unread = u; probe.changes++; probe.mu.Unlock() },
		func() bool { probe.mu.Lock(); defer probe.mu.Unlock(); return probe.focused },
	)
	return w, chat, probe
}

func unreadOf(w *Watcher, space string) int {
	for _, u := range w.Unread() {
		if u.Space == space {
			return u.Count
		}
	}
	return 0
}

func TestWatcherNotifiesNewMessagesFromOthersOnly(t *testing.T) {
	w, chat, probe := newWatchTest(t)
	start := time.Now().Add(-time.Hour)
	chat.post("spaces/dm", "velha", "bruno", "já existia", start)

	w.check(context.Background())
	if got := probe.texts(); len(got) != 0 {
		t.Fatalf("primeira rodada notificou o que já existia: %v", got)
	}

	chat.post("spaces/dm", "m1", "bruno", "oi, tudo bem?", time.Now())
	chat.post("spaces/dm", "m2", "acct-a", "minha resposta", time.Now().Add(time.Millisecond))
	w.check(context.Background())

	if got := probe.texts(); !slices.Equal(got, []string{"oi, tudo bem?"}) {
		t.Fatalf("notificações = %v", got)
	}
	if name := probe.notified[0].Space.DisplayName; name != "Bruno Lima" {
		t.Fatalf("DM sem nome não foi resolvida pelos membros: %q", name)
	}
	if n := unreadOf(w, "spaces/dm"); n != 1 {
		t.Fatalf("não lidas = %d, want 1", n)
	}

	w.check(context.Background())
	if got := probe.texts(); len(got) != 1 {
		t.Fatalf("a mesma mensagem foi notificada de novo: %v", got)
	}
}

func TestWatcherSkipsOpenConversationOnlyWhileFocused(t *testing.T) {
	w, chat, probe := newWatchTest(t)
	w.check(context.Background())
	w.SetOpen("acct-a", "spaces/dm")

	probe.focused = true
	chat.post("spaces/dm", "m1", "bruno", "você está olhando", time.Now())
	w.check(context.Background())
	if len(probe.texts()) != 0 || unreadOf(w, "spaces/dm") != 0 {
		t.Fatalf("notificou a conversa aberta com a janela em foco: %v", probe.texts())
	}

	probe.focused = false
	chat.post("spaces/dm", "m2", "bruno", "janela minimizada", time.Now().Add(time.Millisecond))
	w.check(context.Background())
	if got := probe.texts(); !slices.Equal(got, []string{"janela minimizada"}) {
		t.Fatalf("sem foco devia notificar: %v", got)
	}
}

func TestWatcherOpeningConversationClearsUnread(t *testing.T) {
	w, chat, probe := newWatchTest(t)
	w.check(context.Background())
	chat.post("spaces/dm", "m1", "bruno", "um", time.Now())
	chat.post("spaces/dm", "m2", "bruno", "dois", time.Now().Add(time.Millisecond))
	w.check(context.Background())
	if n := unreadOf(w, "spaces/dm"); n != 2 {
		t.Fatalf("não lidas = %d", n)
	}
	before := probe.changes
	w.SetOpen("acct-a", "spaces/dm")
	if unreadOf(w, "spaces/dm") != 0 || probe.changes != before+1 || len(probe.unread) != 0 {
		t.Fatalf("abrir a conversa não zerou: %+v", w.Unread())
	}
}

func TestWatcherRetriesSpaceAfterError(t *testing.T) {
	w, chat, probe := newWatchTest(t)
	w.check(context.Background())
	chat.post("spaces/dm", "m1", "bruno", "chegou durante a falha", time.Now())
	chat.failSpace = "spaces/dm"
	w.check(context.Background())
	if len(probe.texts()) != 0 {
		t.Fatal("notificou apesar da falha")
	}
	chat.failSpace = ""
	w.check(context.Background())
	if got := probe.texts(); !slices.Equal(got, []string{"chegou durante a falha"}) {
		t.Fatalf("a mensagem se perdeu depois da falha: %v", got)
	}
}

func TestWatcherNewConversationAfterStart(t *testing.T) {
	w, chat, probe := newWatchTest(t)
	w.check(context.Background())
	chat.post("spaces/novo", "m1", "bruno", "primeira mensagem de uma DM nova", time.Now())
	w.check(context.Background())
	if got := probe.texts(); !slices.Equal(got, []string{"primeira mensagem de uma DM nova"}) {
		t.Fatalf("DM criada depois de abrir o app: %v", got)
	}
}

func TestWatcherCapsNotificationsButCountsAll(t *testing.T) {
	w, chat, probe := newWatchTest(t)
	w.check(context.Background())
	for i := range 6 {
		chat.post("spaces/dm", "m"+string(rune('a'+i)), "bruno", "rajada", time.Now().Add(time.Duration(i)*time.Millisecond))
	}
	w.check(context.Background())
	if got := len(probe.texts()); got != notifyPerSpace {
		t.Fatalf("notificações = %d, want %d", got, notifyPerSpace)
	}
	if n := unreadOf(w, "spaces/dm"); n != 6 {
		t.Fatalf("não lidas = %d, want 6", n)
	}
}

func TestWatcherForgetsRemovedAccount(t *testing.T) {
	w, chat, probe := newWatchTest(t)
	w.check(context.Background())
	chat.post("spaces/dm", "m1", "bruno", "oi", time.Now())
	w.check(context.Background())
	if err := w.s.RemoveAccount("acct-a"); err != nil {
		t.Fatal(err)
	}
	w.check(context.Background())
	if len(w.Unread()) != 0 || len(probe.unread) != 0 {
		t.Fatalf("não lidas de conta removida: %+v", w.Unread())
	}
}

func TestMessagesAfterSendsRFC3339Filter(t *testing.T) {
	w, chat, _ := newWatchTest(t)
	at := time.Date(2026, 10, 19, 15, 4, 5, 123000000, time.FixedZone("BRT", -3*3600))
	if _, err := w.s.messagesAfter(context.Background(), "acct-a", "spaces/dm", at); err != nil {
		t.Fatal(err)
	}
	last := chat.requests[len(chat.requests)-1]
	if !strings.Contains(last, "filter=createTime+%3E+%222026-10-19T18%3A04%3A05.123Z%22") || !strings.Contains(last, "orderBy=create_time+ASC") {
		t.Fatalf("consulta = %s", last)
	}
	if _, err := w.s.messagesAfter(context.Background(), "acct-a", "spaces/x?y", at); err == nil {
		t.Fatal("aceitou espaço inválido")
	}
}

func TestNotificationSettings(t *testing.T) {
	s, _ := configuredService(t, &googleStub{})
	if n := s.State().Notifications; !n.On || n.Every != 60 {
		t.Fatalf("padrão = %+v, want ligado a cada 60 s", n)
	}
	for _, every := range []int{0, 1, 14, 61, 3600} {
		if err := s.SetNotifications(Notifications{On: true, Every: every}); err == nil {
			t.Errorf("aceitou intervalo de %d s", every)
		}
	}
	if err := s.SetNotifications(Notifications{On: false, Every: 15}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path, &memoryVault{values: map[string][]byte{}})
	if err != nil {
		t.Fatal(err)
	}
	if n := reopened.State().Notifications; n.On || n.Every != 15 {
		t.Fatalf("depois de reabrir = %+v", n)
	}
}

func TestWatcherRunHonorsOffAndWakes(t *testing.T) {
	w, chat, _ := newWatchTest(t)
	if err := w.s.SetNotifications(Notifications{On: false, Every: 15}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	time.Sleep(50 * time.Millisecond)
	chat.mu.Lock()
	quiet := len(chat.requests)
	chat.mu.Unlock()
	if quiet != 0 {
		t.Fatalf("desligado e mesmo assim consultou: %v", chat.requests)
	}
	if err := w.s.SetNotifications(Notifications{On: true, Every: 15}); err != nil {
		t.Fatal(err)
	}
	w.Wake()
	deadline := time.Now().Add(2 * time.Second)
	for {
		chat.mu.Lock()
		n := len(chat.requests)
		chat.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ligar não acordou o laço")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
}
