package main

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func ternary[T any](cond bool, iftrue T, iffalse T) T {
	if cond {
		return iftrue
	}
	return iffalse
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Choice Widgets")
	myWindow.Resize(fyne.NewSize(400, 400))
	myWindow.SetTitle("Wipr")
	options := []string{}
	driveMap := make(map[string]Win32_Disk)
	for _, v := range List_Drives() {
		for _, p := range v.Partitions {
			tag := fmt.Sprintf("%s %s (%d GB) %s", p.Letter, ternary(p.Label == "", "Local Disk", p.Label), p.Size/1024/1024/1024, ternary(p.Letter == "C:", "(System Drive)", ""))
			options = append(options, tag)
			driveMap[tag] = v
		}
	}
	var str string
	combo := widget.NewSelect(options, func(s string) {
		str = s
	})
	combo.SetSelected(options[0])
	btn := widget.NewButton("Start", func() {
		log.Println(str)
	})
	myWindow.SetContent(container.NewVBox(combo, btn))
	myWindow.ShowAndRun()
}
