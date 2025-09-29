//go:build android

package main

import (
	"fyne.io/fyne/v2"
	"github.com/jaypipes/ghw"
)

func setupSystray(_ fyne.App, _ fyne.Window)                        {}
func ElevateOnLaunch() bool                                         { return true }
func wipePartitions(_ fyne.App, _ *fyne.Window, _ []*ghw.Partition) {}
func setup_creds()                                                  {}

