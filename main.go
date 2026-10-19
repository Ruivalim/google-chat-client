package main

import (
	"context"
	_ "embed"
	"log"
	"path/filepath"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/ruivalim/google-chat-client/internal/chat"
)

//go:embed assets/tray.png
var trayIcon []byte

// quitting deixa a janela fechar de verdade quando o app está saindo.
var quitting atomic.Bool

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
			func(u []chat.Unread) { desktop.unreadChanged(win, u) },
			win.IsFocused,
		)
		go desktop.watcher.Run(context.Background())

		// Com a bandeja, fechar a janela só a esconde: o app segue avisando de
		// mensagens novas. Sem ela (Linux sem AppIndicator), fechar encerra,
		// senão não haveria como voltar à janela.
		show := func() {
			win.Show()
			win.Focus()
		}
		tray, err := mygo.NewTray(mygo.TrayOptions{
			Icon:    trayIcon,
			ToolTip: "Google Chat Client",
			Menu: mygo.NewMenu([]*mygo.MenuItem{
				{Label: "Abrir Google Chat Client", Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
				mygo.Separator(),
				{Label: "Sair", Role: mygo.RoleQuit},
			}),
		})
		if err != nil {
			log.Println("bandeja:", err)
			return
		}
		desktop.tray = tray
		mygo.App.OnWindowAllClosed(func() {})
		win.OnClose(func(e *mygo.CloseEvent) {
			if quitting.Load() {
				return
			}
			e.PreventDefault()
			win.Hide()
		})
		mygo.App.OnActivate(func(visible bool) {
			if !visible {
				show()
			}
		})
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		quitting.Store(true)
		desktop.CancelLogin()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
