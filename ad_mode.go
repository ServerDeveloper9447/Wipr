package main

import (
	"fmt"
	"os/exec"

	"fyne.io/fyne/v2"
)

func CheckPlatformTools() bool {
	adb, err := exec.LookPath("adb")
	if err != nil {
		fmt.Println(err)
		return false
	}
	fastboot, err := exec.LookPath("fastboot")
	if err != nil {
		return false
	}
	fmt.Println(adb, fastboot)
	return adb != "" && fastboot != ""
}

func AndroidMode(wipr fyne.App, window fyne.Window) {
	window.Hide()
	adWindow := wipr.NewWindow("Wipr - Android Mode")
	adWindow.SetTitle("Wipr - Android Mode")
	adWindow.Resize(fyne.NewSize(WIDTH, HEIGHT))
	adWindow.Show()
}