//go:build linux && !android

package platform

import (
	"fmt"
	"os"
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/systray"
	"r00t2.io/gosecret"
)

var (
	secretAttr = map[string]string{
		"appname": "com.usbee.wipr",
	}
	service        *gosecret.Service
	showWinSystray *systray.MenuItem
	quitWinSystray *systray.MenuItem
)

func SetupCreds(onCredsFound func(key string)) {
	var err error
	service, err = gosecret.NewService()
	if err != nil {
		fmt.Println(err)
		return
	}
	unlocked, _, err := service.SearchItems(secretAttr)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(unlocked) > 0 {
		item := unlocked[0]
		onCredsFound(string(item.Secret.Value))
	}
}

func SetKey(s string) {
	if service == nil {
		return
	}
	secret := gosecret.NewSecret(
		service.Session,
		[]byte{},
		[]byte(s),
		"text/plain",
	)
	coll, _ := service.GetCollection("wipr_creds")
	if _, err := coll.CreateItem(
		"Server Key",
		secretAttr,
		secret,
		true,
	); err != nil {
		fmt.Println(err)
	}
}

func DeleteKey() {
	if service == nil {
		return
	}
	unlocked, _, err := service.SearchItems(secretAttr)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(unlocked) > 0 {
		item := unlocked[0]
		item.Delete()
	}
}

func ElevateOnLaunch() bool {
	if os.Geteuid() != 0 {
		exe, _ := os.Executable()
		args := os.Args[1:]

		var envVars []string
		for _, envVar := range []string{"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "XDG_RUNTIME_DIR"} {
			if value := os.Getenv(envVar); value != "" {
				envVars = append(envVars, fmt.Sprintf("%s=%s", envVar, value))
			}
		}

		var cmd *exec.Cmd
		if len(envVars) > 0 {
			finalArgs := []string{"env"}
			finalArgs = append(finalArgs, envVars...)
			finalArgs = append(finalArgs, exe)
			finalArgs = append(finalArgs, args...)
			cmd = exec.Command("pkexec", finalArgs...)
		} else {
			cmd = exec.Command("pkexec", append([]string{exe}, args...)...)
		}

		if err := cmd.Run(); err != nil {
			fmt.Println(err)
			return false
		}
		return false
	}
	return true
}

func SetupSystray(wipr fyne.App, window fyne.Window, images map[string]*fyne.StaticResource, isWiping func() bool) {
	systray.Register(func() {
		icon, ok := images["Icon.ico"]
		if ok {
			systray.SetIcon(icon.StaticContent)
			systray.SetTemplateIcon(icon.StaticContent, icon.StaticContent)
		}
		systray.SetTitle("Wipr v" + wipr.Metadata().Version)
		showWinSystray = systray.AddMenuItem("Show", "Show the Wipr window")
		quitWinSystray = systray.AddMenuItem("Quit", "Quit Wipr")
		go func() {
			for {
				select {
				case <-showWinSystray.ClickedCh:
					if !isWiping() {
						fyne.Do(func() { window.Show() })
					}
				case <-quitWinSystray.ClickedCh:
					fyne.Do(func() { wipr.Quit() })
				}
			}
		}()
		systray.SetTooltip("Wipr v" + wipr.Metadata().Version)
	}, func() {})
}

func DisableSystray() {
	if quitWinSystray != nil {
		quitWinSystray.Disable()
	}
	if showWinSystray != nil {
		showWinSystray.Disable()
	}
}

func EnableSystray() {
	if quitWinSystray != nil {
		quitWinSystray.Enable()
	}
	if showWinSystray != nil {
		showWinSystray.Enable()
	}
}
