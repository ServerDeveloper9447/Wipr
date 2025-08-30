package main

import (
	"fyne.io/fyne/v2/app"
)

func main() {
	List_Drives()
	a := app.New()
	w := a.NewWindow("Hello")
	w.ShowAndRun()
}