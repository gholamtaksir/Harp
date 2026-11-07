// Command harp-gui is the Harp desktop application.
package main

import (
	"embed"
	_ "embed"
	"flag"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/gholamtaksir/Harp/core"
	"github.com/gholamtaksir/Harp/privilege"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

//go:embed all:frontend/dist
var frontendAssets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func init() {
	application.RegisterEvent[core.State](core.EventState)
	application.RegisterEvent[core.Stats](core.EventStats)
	application.RegisterEvent[[]core.LogEntry](core.EventLogs)
	application.RegisterEvent[core.Settings](core.EventSettings)
	application.RegisterEvent[core.ScanState](core.EventScan)
}

func main() {
	waitPID := flag.Int("wait-pid", 0, "wait for this process to exit before starting")
	minimized := flag.Bool("minimized", false, "start hidden in the system tray")
	connect := flag.Bool("connect", false, "connect after starting")
	flag.Parse()

	if *waitPID > 0 {
		// Relaunched with elevated rights: let the previous instance release
		// the single instance lock and the proxy ports first.
		privilege.WaitForExit(*waitPID, 15*time.Second)
	}

	dataDir, err := dataDirectory()
	if err != nil {
		log.Fatal(err)
	}

	var app atomic.Pointer[application.App]
	var tray atomic.Pointer[trayController]
	emit := func(name string, data any) {
		if name == core.EventState {
			if t := tray.Load(); t != nil {
				t.update(data.(core.State))
			}
		}
		if a := app.Load(); a != nil {
			a.Event.Emit(name, data)
		}
	}
	controller, err := core.NewController(core.DefaultPaths(dataDir), emit)
	if err != nil {
		log.Fatal(err)
	}
	service := newHarpService(controller, dataDir)

	var window *application.WebviewWindow
	showWindow := func() {
		if window != nil {
			window.Show()
			window.Restore()
			window.Focus()
		}
	}

	current := application.New(application.Options{
		Name:        "Harp",
		Description: "Cloudflare WARP client with proxy, TUN and routing",
		Icon:        appIcon,
		Services:    []application.Service{application.NewService(service)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(frontendAssets),
		},
		Mac: application.MacOptions{
			// Harp keeps running in the menu bar when the window is closed.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			AdditionalBrowserArgs: browserArgs(),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "io.github.gholamtaksir.harp" + instanceSuffix,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				showWindow()
			},
		},
		OnShutdown: controller.Shutdown,
	})
	app.Store(current)

	settings := controller.Settings()
	window = current.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Harp",
		Width:            1220,
		Height:           800,
		MinWidth:         1000,
		MinHeight:        660,
		Hidden:           *minimized || settings.App.StartMinimized,
		BackgroundColour: application.NewRGB(9, 11, 17),
		URL:              "/",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if service.quitting.Load() {
			return
		}
		if controller.Settings().App.CloseToTray {
			event.Cancel()
			window.Hide()
			return
		}
		service.quit()
	})

	service.attach(current, window)
	tray.Store(newTray(current, controller, service, showWindow))
	tray.Load().update(controller.State())

	current.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if *connect || settings.App.ConnectOnLaunch {
			go func() { _ = controller.Connect() }()
		}
	})

	if err := current.Run(); err != nil {
		log.Fatal(err)
	}
}

// dataDirectory returns where settings and the account are stored. It can be
// overridden with HARP_DATA_DIR, e.g. for a portable installation.
func dataDirectory() (string, error) {
	if dir := os.Getenv("HARP_DATA_DIR"); dir != "" {
		return dir, os.MkdirAll(dir, 0o700)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Harp")
	return dir, os.MkdirAll(dir, 0o700)
}
