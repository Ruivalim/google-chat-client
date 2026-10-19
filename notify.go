package main

import (
	"errors"
	"github.com/ruivalim/google-chat-client/locales"
	"log"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/ruivalim/google-chat-client/internal/chat"
)

// OpenConversation pede à interface para abrir a conversa de uma notificação.
var OpenConversation = mygo.NewEvent[chat.Activity]("open:conversation")

// UnreadChanged leva as não lidas de todas as contas sempre que mudam.
var UnreadChanged = mygo.NewEvent[[]chat.Unread]("unread:changed")

func (d *Desktop) notify(win *mygo.Window, a chat.Activity) {
	if !mygo.NotificationsSupported() {
		return
	}
	state := d.service.State()
	title := senderName(a.Message.Sender)
	if a.Space.SpaceType != "DIRECT_MESSAGE" && a.Space.DisplayName != "" {
		title = locales.NotifyInSpace(title, a.Space.DisplayName)
	}
	// Com várias contas, o título diz de qual delas é a conversa.
	if len(state.Accounts) > 1 {
		for _, acc := range state.Accounts {
			if acc.ID == a.AccountID {
				title += " · " + firstNonEmpty(acc.Domain, acc.Email)
			}
		}
	}
	// Mesmo ID por conversa: a mensagem nova substitui o aviso anterior dela
	// em vez de empilhar.
	n := mygo.NewNotification(mygo.NotificationOptions{
		ID: a.AccountID + "|" + a.Space.Name, Group: a.Space.Name,
		Title: title, Body: preview(a.Message),
	})
	n.OnClick(func() {
		win.Show()
		win.Focus()
		_ = OpenConversation.Emit(win, a)
		n.Close()
	})
	if err := n.Show(); err != nil {
		log.Printf("notification: %v", err)
	}
}

func senderName(u chat.User) string {
	return firstNonEmpty(u.DisplayName, u.Email, locales.NotifyNewMessage())
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func preview(m chat.Message) string {
	text := strings.Join(strings.Fields(m.Text), " ")
	if text == "" {
		if len(m.Attachment) > 0 {
			return locales.NotifySentFile(firstNonEmpty(m.Attachment[0].ContentName, locales.NotifyAnAttachment()))
		}
		return locales.NotifyOpenToRead()
	}
	if utf8.RuneCountInString(text) > 180 {
		return string([]rune(text)[:179]) + "…"
	}
	return text
}

// unreadChanged leva as não lidas para a janela e para a dica da bandeja.
func (d *Desktop) unreadChanged(win *mygo.Window, list []chat.Unread) {
	_ = UnreadChanged.Emit(win, list)
	if d.tray == nil {
		return
	}
	total := 0
	for _, u := range list {
		total += u.Count
	}
	d.tray.SetToolTip(locales.UnreadTooltip(total))
}

func (d *Desktop) Unread() []chat.Unread {
	if d.watcher == nil {
		return []chat.Unread{}
	}
	return d.watcher.Unread()
}

// SetOpenConversation avisa qual conversa está na tela; "" quando nenhuma.
func (d *Desktop) SetOpenConversation(accountID, space string) {
	if d.watcher != nil {
		d.watcher.SetOpen(accountID, space)
	}
}

func (d *Desktop) SetNotifications(n chat.Notifications) (chat.State, error) {
	err := d.service.SetNotifications(n)
	if err == nil && d.watcher != nil {
		d.watcher.Wake()
	}
	return d.State(), err
}

func (d *Desktop) CheckIntervals() []int { return chat.CheckIntervals }

// SetLanguage troca o idioma do app inteiro: as mensagens do Go, os menus e
// a página. "" volta a seguir o sistema.
func (d *Desktop) SetLanguage(tag string) (chat.State, error) {
	if tag != "" && !slices.Contains(mygo.App.Languages(), tag) {
		return d.State(), errors.New(locales.ErrLanguage())
	}
	if err := d.service.SetLanguage(tag); err != nil {
		return d.State(), err
	}
	mygo.App.SetLocale(tag)
	return d.State(), nil
}
