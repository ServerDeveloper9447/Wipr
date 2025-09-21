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
	MinimizeOnClose bool   `json:"minimizeOnClose"`
	EnterpriseMode  bool   `json:"entMode"`
	HostUrl         string `json:"hosturl"`
	PassKey         string `json:"passkey"`
}

func ternary[T any](cond bool, iftrue T, iffalse T) T {
	if cond {
		return iftrue
	}
	return iffalse
}

const (
	WIDTH  = 700
	HEIGHT = 400
)

func main() {
	var config Config
	wipr := app.New()
	window := wipr.NewWindow("Wipr")
	window.Resize(fyne.NewSize(WIDTH, HEIGHT))
	window.SetTitle("Wipr")
	window.SetMaster()
	var modalHidden *bool
	t := true
	modalHidden = &t
	window.SetCloseIntercept(func() {
		if config.MinimizeOnClose {
			window.Hide()
			return
		}
		var modal *widget.PopUp
		box := container.New(NewCustomPaddedBoxLayout(15, 15), container.NewPadded(container.NewVBox(
			widget.NewLabel("Are you sure you want to quit?"),
			container.NewGridWithColumns(2, widget.NewButton("Yes", func() {
				window.Close()
			}), widget.NewButton("Cancel", func() {
				modal.Hide()
				*modalHidden = true
			})))),
		)
		modal = widget.NewModalPopUp(box, window.Canvas())
		if *modalHidden {
			modal.Show()
			*modalHidden = false
		}
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
			key := widget.NewEntry()
			key.Password = true
			key.OnChanged = func(s string) {
				s = strings.ReplaceAll(s, " ", "")
				key.SetText(s)
			}
			btn := widget.NewButtonWithIcon("Connect", theme.CheckButtonCheckedIcon(), func() {
				config.EnterpriseMode = true
				config.HostUrl = hostUrl.Text
				config.PassKey = key.Text
				verifyBtn.Show()
				modal.Hide()
			})
			btn.Importance = widget.HighImportance
			if config.EnterpriseMode {
				hostUrl.Enable()
				key.Enable()
				btn.Enable()
			} else {
				hostUrl.Disable()
				key.Disable()
				btn.Disable()
			}
			checkB := widget.NewCheck("Enterprise Mode", func(b bool) {
				if b {
					hostUrl.Enable()
					key.Enable()
					btn.Enable()
					verifyBtn.Show()
				} else {
					hostUrl.Disable()
					key.Disable()
					btn.Disable()
					verifyBtn.Hide()
				}
			})
			checkB.Checked = config.EnterpriseMode
			mOC := widget.NewCheck("Minimize on close", func(b bool) {
				config.MinimizeOnClose = b
			})
			mOC.Checked = config.MinimizeOnClose
			box := container.New(NewCustomPaddedBoxLayout(5, 5),
				container.NewPadded(
					container.NewVBox(
						mOC,
						checkB,
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
			modal.Resize(fyne.NewSize(WIDTH-300, HEIGHT-100))
			modal.Show()
		}),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			url, _ := url.Parse("https://wipr.vercel.app")
			wipr.OpenURL(url)
		}),
	)
	warningLabel := widget.NewLabel("Data deleted by Wipr is unrecoverable. Data destruction due to user error is not the responsibility of the developers.")
	warningLabel.Wrapping = fyne.TextWrapWord
	warningLabel.TextStyle = fyne.TextStyle{Bold: true, Underline: true}
	warningLabel.Alignment = fyne.TextAlignCenter
	btmToolbar := container.NewVBox(
		warningLabel,
		container.NewHBox(
			layout.NewSpacer(),
			widget.NewLabel("v"+wipr.Metadata().Version),
		))
	drives := List_Drives()
	partitions := List_Partitions()
	var wipeBtn *widget.Button
	selectOptions := widget.NewSelect(drives, func(s string) {
		if wipeBtn != nil {
			wipeBtn.Enable()
		}
	})
	typeOptions := widget.NewSelect([]string{"By Disk Drive", "By Partitions"}, func(s string) {
		selectOptions.ClearSelected()
		if wipeBtn != nil {
			wipeBtn.Disable()
		}
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

	bg := canvas.NewRectangle(color.Transparent)
	bg.SetMinSize(fyne.NewSize(WIDTH-100, HEIGHT))
	var box *fyne.Container
	wipeBtn = widget.NewButtonWithIcon("Wipe", theme.DeleteIcon(), func() {
		fmt.Println("Wiping methods...")
		ok, err := Wipr(wipr, &window, box, Data{Mode: typeOptions.Selected, Path: selectOptions.Selected})
		if !ok {
			fmt.Println(err)
		}
	})
	box = container.NewVBox(wiprText,
		spacer,
		typeOptions,
		selectOptions,
		layout.NewSpacer(),
		wipeBtn,
		verifyBtn,
	)
	wipeBtn.Importance = widget.DangerImportance
	ctn := container.New(
		NewCustomPaddedBoxLayout(15, 0),
		container.NewPadded(box),
	)

	boxWithBg := container.NewStack(bg, ctn)
	content := container.NewBorder(toolbar, btmToolbar, nil, nil, boxWithBg)
	m := fyne.NewMenu("Wipr",
		fyne.NewMenuItem("Show", func() {
			window.Show()
		}),
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
