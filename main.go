package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Config struct {
	EnterpriseMode bool `json:"entMode"`
}

func ternary[T any](cond bool, iftrue T, iffalse T) T {
	if cond {
		return iftrue
	}
	return iffalse
}

func main() {
	wipr := app.New()
	window := wipr.NewWindow("Wipr")
	window.Resize(fyne.NewSize(800, 400))
	window.SetTitle("Wipr")
	window.SetFixedSize(true)
	window.SetMaster()
	window.SetCloseIntercept(func() {
		cWindow := wipr.NewWindow("Confirm quit")
		cWindow.SetTitle("Quit?")
		cWindow.SetFixedSize(true)
		box := container.NewVBox(
			widget.NewLabel("Are you sure you want to quit?"),
			container.NewGridWithColumns(2, widget.NewButton("Yes", func() {
				window.Close()
			}), widget.NewButton("Cancel", func() {
				cWindow.Close()
			})),
		)
		cWindow.SetContent(box)
		cWindow.Show()
		cWindow.RequestFocus()
		cWindow.CenterOnScreen()
	})
	toolbar := widget.NewToolbar(
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.SettingsIcon(), func() {}),
		widget.NewToolbarAction(theme.HelpIcon(), func() {}),
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
	box := container.NewVBox(
		widget.NewLabelWithStyle("Wipr", fyne.TextAlignCenter, fyne.TextStyle{Bold: true, Monospace: true}),
		typeOptions,
		selectOptions,
	)
	content := container.NewBorder(toolbar, nil, nil, nil, box)
	window.CenterOnScreen()
	window.SetContent(content)
	window.ShowAndRun()
}
