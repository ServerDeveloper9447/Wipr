package platform

import "fyne.io/fyne/v2"

type Platform interface {
	ElevateOnLaunch() bool
	SetupSystray(app fyne.App, window fyne.Window, images map[string]*fyne.StaticResource, isWiping func() bool)
	SetupCreds(onCredsFound func(key string))
}
