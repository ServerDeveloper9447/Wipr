//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/jaypipes/ghw"
)

var (
	driveMap     = make(map[string]*ghw.Disk)
	partitionMap = make(map[string]*ghw.Partition)
)

func List_Drives() []string {
	block, _ := ghw.Block()
	drives := []string{}
	for _, d := range block.Disks {
		driveMap[d.Model] = d
		drives = append(drives, d.Model)
	}
	return drives
}

func List_Partitions() []string {
	block, _ := ghw.Block()
	paritions := []string{}
	for _, d := range block.Disks {
		for _, p := range d.Partitions {
			paritions = append(paritions, fmt.Sprintf("%s %s", p.Name, d.Model))
			partitionMap[fmt.Sprintf("%s %s", p.Name, d.Model)] = p
		}
	}
	return paritions
}

type Data struct {
	Mode string
	Path string
}

func Wipr(app fyne.App, window *fyne.Window, box *fyne.Container, data Data) (success bool, err error) {
	fmt.Println(data)
	if data.Mode != "By Partitions" && data.Mode != "By Disk Drive" {
		return false, errors.New("invalid mode")
	}
	partition := partitionMap[data.Path]
	if partition == nil {
		return false, errors.New("invalid partition")
	}
	files := []string{}
	err = filepath.Walk(partition.MountPoint, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return false, err
	}
	box.RemoveAll()
	textArea := widget.NewLabel("")
	prg := widget.NewProgressBar()
	go func() {
		l := len(files)
		for i, f := range files {
			time.Sleep(1 * time.Second)
			fyne.DoAndWait(func() {
				prg.SetValue(float64(i+1) / float64(l))
				textArea.SetText(f)
			})
		}
	}()
	box.Add(prg)
	box.Add(textArea)
	box.Refresh()
	return false, nil
}
