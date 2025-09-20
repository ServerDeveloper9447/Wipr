package main

import (
	"fmt"
	"image/color"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Config struct {
	EnterpriseMode bool   `json:"entMode"`
	HostUrl        string `json:"hosturl"`
	PassKey        string `json:"passkey"`
}

func ternary[T any](cond bool, iftrue T, iffalse T) T {
	if cond {
		return iftrue
	}
	return iffalse
}

func main() {
	var config Config
	wipr := app.New()
	window := wipr.NewWindow("Wipr")
	window.Resize(fyne.NewSize(700, 400))
	window.SetTitle("Wipr")
	window.SetMaster()
	window.SetCloseIntercept(func() {
		var modal *widget.PopUp
		box := container.New(NewCustomPaddedBoxLayout(15, 15), container.NewPadded(container.NewVBox(
			widget.NewLabel("Are you sure you want to quit?"),
			container.NewGridWithColumns(2, widget.NewButton("Yes", func() {
				window.Close()
			}), widget.NewButton("Cancel", func() {
				modal.Hide()
			})))),
		)
		modal = widget.NewModalPopUp(box, window.Canvas())
		modal.Show()
	})
	verifyBtn := widget.NewButtonWithIcon("Verify", theme.CheckButtonIcon(), func() {
		fmt.Println("Verifying methods...")
	})
	if !config.EnterpriseMode {
		verifyBtn.Hide()
	}
	toolbar := widget.NewToolbar(
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.SettingsIcon(), func() {
			var modal *widget.PopUp

			hostUrl := widget.NewEntry()
			hostUrl.PlaceHolder = "http://127.0.0.1:56009"
			hostUrl.OnChanged = func(s string) {
				s = strings.ReplaceAll(s, " ", "")
				hostUrl.SetText(s)
			}
			hostUrl.Disable()
			key := widget.NewEntry()
			key.Password = true
			key.OnChanged = func(s string) {
				s = strings.ReplaceAll(s, " ", "")
				key.SetText(s)
			}
			key.Disable()
			btn := widget.NewButtonWithIcon("Connect", theme.CheckButtonCheckedIcon(), func() {
				config.EnterpriseMode = true
				config.HostUrl = hostUrl.Text
				config.PassKey = key.Text
				verifyBtn.Show()
				modal.Hide()
			})
			btn.Importance = widget.HighImportance
			box := container.New(NewCustomPaddedBoxLayout(5, 5),
				container.NewPadded(
					container.NewVBox(
						widget.NewCheck("Enterprise Mode", func(b bool) {
							if b {
								hostUrl.Enable()
								key.Enable()
							} else {
								hostUrl.Disable()
								key.Disable()
							}
						}),
						widget.NewLabel("Host URL"),
						hostUrl,
						widget.NewLabel("Connection Key"),
						key,
						layout.NewSpacer(),
						container.NewGridWithColumns(2,
							btn,
							widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
								modal.Hide()
							}),
						),
					),
				),
			)
			modal = widget.NewModalPopUp(box, window.Canvas())
			modal.Resize(fyne.NewSize(400, 300))
			modal.Show()
		}),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			url, _ := url.Parse("https://github.com/US-BEE-SIH/Wipr")
			wipr.OpenURL(url)
		}),
	)
	btmToolbar := container.NewHBox(
		layout.NewSpacer(),
		widget.NewLabel("v" + wipr.Metadata().Version),
	)
	drives := List_Drives()
	partitions := List_Partitions()
	selectOptions := widget.NewSelect(drives, func(s string) {})
	typeOptions := widget.NewSelect([]string{"By Disk Drive", "By Partitions"}, func(s string) {
		selectOptions.ClearSelected()
		selectOptions.SetOptions(ternary(s == "By Disk Drive", drives, partitions))
	})
	typeOptions.SetSelectedIndex(0)
	selectOptions.SetSelectedIndex(0)
	wiprText := canvas.NewText("Wipr", theme.Color(theme.ColorNameForeground))
	wiprText.TextSize = 20
	wiprText.Alignment = fyne.TextAlignCenter
	wiprText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(0, 30))

	warningLabel := widget.NewLabel("Data deleted by Wipr is unrecoverable. Data destruction due to user error is not the responsibility of the developers.")
	warningLabel.Wrapping = fyne.TextWrapWord
	warningLabel.TextStyle = fyne.TextStyle{Bold: true, Underline: true}
	warningLabel.Alignment = fyne.TextAlignCenter

	bg := canvas.NewRectangle(color.Transparent)
	bg.SetMinSize(fyne.NewSize(600, 400))
	wipeBtn := widget.NewButtonWithIcon("Wipe", theme.DeleteIcon(), func() {
		fmt.Println("Wiping methods...")
	})
	wipeBtn.Importance = widget.DangerImportance

	box := container.New(
		NewCustomPaddedBoxLayout(15, 0),
		container.NewPadded(container.NewVBox(wiprText,
			spacer,
			typeOptions,
			selectOptions,
			layout.NewSpacer(),
			wipeBtn,
			verifyBtn,
			warningLabel,
		),
		),
	)

	boxWithBg := container.NewStack(bg, box)
	content := container.NewBorder(toolbar, btmToolbar, nil, nil, boxWithBg)
	m := fyne.NewMenu("Wipr",
		fyne.NewMenuItem("Quit", func() {
			window.Close()
		}))
	if desk, ok := wipr.(desktop.App); ok {
		desk.SetSystemTrayMenu(m)
	}

	window.CenterOnScreen()
	window.SetContent(content)
	window.RequestFocus()
	window.ShowAndRun()
}
