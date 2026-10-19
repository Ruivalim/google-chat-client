package main

import (
	"context"
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/ruivalim/google-chat-client/internal/chat"
)

func main() {
	// Sem SetName: o nome vem do pacote, e o `mygo dev` chama o app de
	// "Google Chat Client Dev". Assim dev e app instalado têm accounts.json
	// separados, cada um casado com o seu serviço no chaveiro.
	desktop := &Desktop{}
	mygo.Bind(desktop)
	mygo.App.WhenReady(func() {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			log.Fatal(err)
		}
		desktop.service, err = chat.Open(filepath.Join(dir, "accounts.json"), systemVault{})
		if err != nil {
			log.Fatal(err)
		}
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu}, {Role: mygo.RoleWindowMenu},
		}))
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "Google Chat Client", Width: 1240, Height: 800,
			MinWidth: 720, MinHeight: 540, URL: "/",
			BackgroundColor: "light-dark(#f7f8f7, #181b1a)",
		})
		win.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
			e.PreventDefault()
			go func() { _ = desktop.OpenLink(e.URL) }()
		})
		desktop.watcher = chat.NewWatcher(desktop.service,
			func(a chat.Activity) { desktop.notify(win, a) },
			func(u []chat.Unread) { _ = UnreadChanged.Emit(win, u) },
			win.IsFocused,
		)
		go desktop.watcher.Run(context.Background())
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { desktop.CancelLogin() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
