package main

import (
	"fmt"
	"log"
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
		title += " em " + a.Space.DisplayName
	}
	// Com várias contas, o título diz de qual delas é a conversa.
	if len(state.Accounts) > 1 {
		for _, acc := range state.Accounts {
			if acc.ID == a.AccountID {
				title += " · " + firstNonEmpty(acc.Domain, acc.Email)
			}
		}
	}
	n := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: preview(a.Message)})
	n.OnClick(func() {
		win.Show()
		win.Focus()
		_ = OpenConversation.Emit(win, a)
		n.Close()
	})
	if err := n.Show(); err != nil {
		log.Printf("notificação: %v", err)
	}
}

func senderName(u chat.User) string {
	return firstNonEmpty(u.DisplayName, u.Email, "Mensagem nova")
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
			return fmt.Sprintf("Enviou %s", firstNonEmpty(m.Attachment[0].ContentName, "um anexo"))
		}
		return "Abra a conversa para ver a mensagem"
	}
	if utf8.RuneCountInString(text) > 180 {
		return string([]rune(text)[:179]) + "…"
	}
	return text
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
