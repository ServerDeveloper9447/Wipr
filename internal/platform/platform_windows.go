//go:build windows

package platform

import (
	"fmt"
	"os"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/systray"
	"github.com/danieljoos/wincred"
	"golang.org/x/sys/windows"
)

var (
	showWinSystray *systray.MenuItem
	quitWinSystray *systray.MenuItem
)

func SetupCreds(onCredsFound func(key string)) {
	key, err := wincred.GetGenericCredential("Wipr/ServerKey")
	if err != nil {
		return
	}
	onCredsFound(string(key.CredentialBlob))
}

func ElevateOnLaunch() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer token.Close()
	var elevation uint32
	var retLen uint32
	err = windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &retLen)
	if err != nil {
		fmt.Println(err)
		return false
	}
	if elevation == 0 {
		shell32 := windows.NewLazyDLL("shell32.dll")
		procShellExecute := shell32.NewProc("ShellExecuteW")
		exe, _ := os.Executable()
		ret, _, err := procShellExecute.Call(
			0,
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("runas"))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(exe))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(""))),
			0,
			0,
		)
		if ret > 32 {
			return false
		}
		fmt.Println(err)
		user32 := windows.NewLazyDLL("user32.dll")
		procMessageBoxW := user32.NewProc("MessageBoxW")
		procMessageBoxW.Call(
			uintptr(0),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Wipr needs admin access to launch"))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Failed to launch Wipr"))),
			uintptr(0),
		)
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
