package main

import "fyne.io/fyne/v2"

func NewCustomPaddedBoxLayout(pW float32, pH float32) fyne.Layout {
	return &paddedHBoxLayout{PadWidth: pW, PadHeight: pH}
}

type paddedHBoxLayout struct {
	PadWidth  float32
	PadHeight float32
}

func (p *paddedHBoxLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	// This layout is for a single object to be padded horizontally
	obj := objects[0]
	obj.Move(fyne.NewPos(p.PadWidth, p.PadHeight))
	obj.Resize(fyne.NewSize(size.Width-2*p.PadWidth, size.Height-2*p.PadHeight))
}

func (p *paddedHBoxLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	min := objects[0].MinSize()
	return fyne.NewSize(min.Width+2*p.PadWidth, min.Height+2*p.PadHeight)
}
