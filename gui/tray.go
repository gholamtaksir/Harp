package main

import (
	_ "embed"
	"runtime"
	"sync"

	"github.com/gholamtaksir/Harp/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed assets/tray-on.png
var trayOnIcon []byte

//go:embed assets/tray-off.png
var trayOffIcon []byte

//go:embed assets/tray-template.png
var trayTemplateIcon []byte

// trayController keeps the system tray icon and menu in sync with the
// connection state.
type trayController struct {
	mu         sync.Mutex
	tray       *application.SystemTray
	controller *core.Controller
	status     *application.MenuItem
	toggle     *application.MenuItem
	proxyMode  *application.MenuItem
	tunMode    *application.MenuItem
	connected  *bool
}

func newTray(app *application.App, controller *core.Controller, service *HarpService, showWindow func()) *trayController {
	t := &trayController{tray: app.SystemTray.New(), controller: controller}

	menu := app.NewMenu()
	t.status = menu.Add("Disconnected").SetEnabled(false)
	menu.AddSeparator()
	t.toggle = menu.Add("Connect").OnClick(func(*application.Context) {
		go func() {
			if controller.State().Status == core.StatusConnected {
				_ = controller.Disconnect()
			} else {
				_ = controller.Connect()
			}
		}()
	})
	menu.AddSeparator()
	t.proxyMode = menu.AddRadio("Proxy mode", true).OnClick(func(*application.Context) {
		go func() { _ = controller.SetMode(core.ModeProxy) }()
	})
	t.tunMode = menu.AddRadio("TUN mode (whole system)", false).OnClick(func(*application.Context) {
		go func() { _ = controller.SetMode(core.ModeTun) }()
	})
	menu.AddSeparator()
	menu.Add("Open Harp").OnClick(func(*application.Context) { showWindow() })
	menu.Add("Quit Harp").OnClick(func(*application.Context) { service.quit() })

	t.tray.SetMenu(menu)
	t.tray.OnClick(showWindow)
	if runtime.GOOS == "darwin" {
		t.tray.SetTemplateIcon(trayTemplateIcon)
	}
	return t
}

func (t *trayController) update(state core.State) {
	t.mu.Lock()
	defer t.mu.Unlock()

	connected := state.Status == core.StatusConnected
	if runtime.GOOS != "darwin" && (t.connected == nil || *t.connected != connected) {
		if connected {
			t.tray.SetIcon(trayOnIcon)
		} else {
			t.tray.SetIcon(trayOffIcon)
		}
	}
	t.connected = &connected

	mode := "Proxy"
	if state.Mode == core.ModeTun {
		mode = "TUN"
	}
	var label string
	switch state.Status {
	case core.StatusConnected:
		label = "Connected · " + mode
		if state.Trace.Warp == "plus" {
			label += " · WARP+"
		}
	case core.StatusConnecting:
		label = "Connecting…"
	case core.StatusDisconnecting:
		label = "Disconnecting…"
	case core.StatusError:
		label = "Connection failed"
	default:
		label = "Disconnected"
	}
	t.status.SetLabel(label)
	t.tray.SetTooltip("Harp · " + label)

	busy := state.Status == core.StatusConnecting || state.Status == core.StatusDisconnecting
	if connected {
		t.toggle.SetLabel("Disconnect")
	} else {
		t.toggle.SetLabel("Connect")
	}
	t.toggle.SetEnabled(!busy)
	t.proxyMode.SetChecked(state.Mode != core.ModeTun)
	t.tunMode.SetChecked(state.Mode == core.ModeTun)
}
