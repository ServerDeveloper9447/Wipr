package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func StartAdb() error {
	_, err := exec.LookPath("adb")
	if err != nil {
		return err
	}
	_, err = exec.LookPath("fastboot")
	if err != nil {
		return err
	}
	cmd := exec.Command("adb", "start-server")
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

type Device struct {
	ID    string
	State string
	Model string
	Brand string
}

func getAdbDeviceInfo(serial string, state string) (Device, error) {
	if state != "device" {
		return Device{}, errors.New("cannot get info of device in offline state")
	}
	cmd := exec.Command("adb", "-s", serial, "shell", "getprop", "ro.product.brand")
	var brand bytes.Buffer
	cmd.Stdout = &brand
	err := cmd.Run()
	if err != nil {
		return Device{}, err
	}
	cmd = exec.Command("adb", "-s", serial, "shell", "getprop", "ro.product.model")
	var model bytes.Buffer
	cmd.Stdout = &model
	err = cmd.Run()
	if err != nil {
		return Device{}, err
	}
	return Device{ID: serial, State: state, Model: strings.Trim(model.String(), "\n"), Brand: strings.Trim(brand.String(), "\n")}, nil
}

func getAdbDevices() (map[string]Device, error) {
	cmd := exec.Command("adb", "devices")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	devices := make(map[string]Device)
	if len(lines) > 1 {
		for _, line := range lines[1:] {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				info, err := getAdbDeviceInfo(parts[0], parts[1])
				if err == nil {
					devices[fmt.Sprintf("%s %s", info.Brand, info.Model)] = info
				}
			}
		}
	}
	return devices, nil
}

func addAdbDevice(host string, code string) error {
	cmd := exec.Command("adb", "pair", host, code)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func AndroidMode(wipr fyne.App, window fyne.Window) {
	if drv, ok := wipr.Driver().(desktop.Driver); ok {
		w := drv.CreateSplashWindow()
		w.SetContent(container.NewVBox(
			layout.NewSpacer(),
			widget.NewLabelWithStyle("Checking platform tools...", fyne.TextAlignCenter, fyne.TextStyle{}),
			layout.NewSpacer(),
		))
		w.Resize(fyne.NewSize(300, 200))
		w.Show()
		err := StartAdb()
		if err != nil {
			w.Close()
			window.Show()
			dialog.ShowError(errors.New("adb or fastboot not found. please check if you have platform-tools installed and try again. additionally try `adb start-server` in your terminal"), window)
			return
		}
		w.Close()
	}
	window.Hide()
	adWindow := wipr.NewWindow("Wipr - Android Mode")
	adWindow.SetTitle("Wipr - Android Mode")
	adWindow.Resize(fyne.NewSize(WIDTH-100, HEIGHT))
	adWindow.SetFixedSize(true)
	toolbar := widget.NewToolbar(
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.DesktopIcon(), func() {
			dialog.ShowConfirm("Exit", "Are you sure want to exit Android Mode?", func(b bool) {
				if b {
					adWindow.Close()
					window.Show()
				}
			}, adWindow)
		}),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			url, _ := url.Parse(fmt.Sprintf("%s/android", WEBSITE_URL))
			wipr.OpenURL(url)
		}),
	)
	devices, err := getAdbDevices()
	if err != nil {
		dialog.NewError(errors.New("couldn't get devices connected to adb"), adWindow)
	}
	deviceOptions := []string{}
	for k := range devices {
		deviceOptions = append(deviceOptions, k)
	}
	var deviceSelect *widget.Select
	btn := widget.NewButtonWithIcon("Wipe", theme.DeleteIcon(), func() {
		if deviceSelect == nil {
			return
		}
		_ = devices[deviceSelect.Selected]
	})
	btn.Disable()
	btn.Importance = widget.DangerImportance
	deviceSelect = widget.NewSelect(append(deviceOptions, "All Devices"), func(s string) {
		if len(deviceOptions) != 0 {
			btn.Enable()
		}
	})
	deviceSelect.PlaceHolder = "Select Device"
	box := container.NewVBox(
		widget.NewLabelWithStyle("Wipr Android Mode", fyne.TextAlignCenter, fyne.TextStyle{
			Monospace: true,
			Bold:      true,
		}),
		container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("", icons["devices.svg"], func() {
			var modal *widget.PopUp
			host := widget.NewEntry()
			pairCode := widget.NewEntry()
			box := container.New(
				NewCustomPaddedBoxLayout(15, 15),
				container.NewPadded(container.NewVBox(
					widget.NewLabel("Hostname[:port]"),
					host,
					widget.NewLabel("Pairing Code"),
					pairCode,
					layout.NewSpacer(),
					container.NewGridWithColumns(2,
						widget.NewButtonWithIcon("Pair", theme.CheckButtonCheckedIcon(), func() {
							if host.Text == "" || pairCode.Text == "" {
								dialog.ShowError(errors.New("host or pair code cannot be empty"), adWindow)
								return
							}
							if len(pairCode.Text) != 6 {
								dialog.ShowError(errors.New("pair code must be 6 digits long positive number"), adWindow)
								return
							}
							var code int
							var err error
							if code, err = strconv.Atoi(pairCode.Text); err != nil {
								dialog.ShowError(errors.New("pair code must be a 6 digit long positive number"), adWindow)
								return
							}
							if code < 0 {
								dialog.ShowError(errors.New("pair code must be a 6 digit long positive number"), adWindow)
								return
							}
							err = addAdbDevice(host.Text, pairCode.Text)
							if err != nil {
								dialog.ShowError(err, adWindow)
								return
							}
							devices, err = getAdbDevices()
							if err != nil {
								dialog.ShowError(err, adWindow)
								return
							}
							deviceOptions = []string{}
							for k := range devices {
								deviceOptions = append(deviceOptions, k)
							}
							deviceSelect.Options = append(deviceOptions, "All Devices")
							if modal != nil {
								modal.Hide()
							}
						}),
						widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
							if modal != nil {
								modal.Hide()
							}
						}),
					),
				)),
			)
			modal = widget.NewModalPopUp(box, adWindow.Canvas())
			modal.Resize(fyne.NewSize(WIDTH-300, HEIGHT-100))
			modal.Show()
		}), deviceSelect),
		layout.NewSpacer(),
		btn,
	)
	border := container.NewBorder(toolbar, container.NewHBox(
		layout.NewSpacer(),
		widget.NewLabel("v"+wipr.Metadata().Version),
	), nil, nil, box)
	content := container.New(
		NewCustomPaddedBoxLayout(15, 0),
		container.NewPadded(border),
	)
	adWindow.SetCloseIntercept(func() {
		dialog.ShowConfirm("Exit", "Are you sure want to quit?", func(b bool) {
			if b {
				wipr.Quit()
			}
		}, adWindow)
	})
	adWindow.SetContent(content)
	adWindow.CenterOnScreen()
	adWindow.RequestFocus()
	adWindow.Show()
}
