//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/danieljoos/wincred"
	"github.com/jaypipes/ghw"
	"golang.org/x/sys/windows"
)

var (
	driveMap     = make(map[string]*ghw.Disk)
	partitionMap = make(map[string]*ghw.Partition)
)

type Data struct {
	Mode string
	Path string
}

func setup_creds() {
	hosturl, err := wincred.GetGenericCredential("Wipr/ServerHost")
	if err != nil {
		return
	}
	key, err := wincred.GetGenericCredential("Wipr/ServerKey")
	if err != nil {
		return
	}
	config.HostUrl = string(hosturl.CredentialBlob)
	config.PassKey = string(key.CredentialBlob)
	config.EnterpriseMode = true
}

func wipePartitions(app fyne.App, window *fyne.Window, partitions []*ghw.Partition) (success bool, err error) {
	isWiping = true
	(*window).Hide()

	progressWindow := app.NewWindow("Wiping in progress")

	partitionsLabel := widget.NewLabel("")
	sizeLabel := widget.NewLabel("")
	textArea := widget.NewLabel("")
	textArea.Wrapping = fyne.TextWrapBreak
	prg := widget.NewProgressBar()

	if len(partitions) <= 1 {
		partitionsLabel.Hide()
	}

	pauseChan := make(chan bool, 1)
	cancelChan := make(chan struct{})
	cancelFunc := func() {
		pauseChan <- true
		dialog.ShowConfirm("Cancel?", "Are you sure you want to cancel?", func(confirm bool) {
			if confirm {
				close(cancelChan)
			} else {
				pauseChan <- false
			}
		}, progressWindow)
	}
	cancelButton := widget.NewButton("Cancel", cancelFunc)

	progressBox := container.NewVBox(widget.NewLabel("Wiping..."), partitionsLabel, sizeLabel, prg, textArea, layout.NewSpacer(), cancelButton)
	progressWindow.SetContent(progressBox)
	progressWindow.Resize(fyne.NewSize(400, 200))
	progressWindow.SetFixedSize(true)
	progressWindow.CenterOnScreen()
	progressWindow.SetCloseIntercept(cancelFunc)

	go func() {
		defer func() {
			fyne.Do(func() {
				isWiping = false
				(*window).Show()
				progressWindow.Close()
			})
		}()

		var accumulatedSize uint64 = 0
		var totalPartitionSize uint64 = 0
		var totalUsedBytes uint64 = 0

		for _, p := range partitions {
			totalPartitionSize += p.SizeBytes
			var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
			err := windows.GetDiskFreeSpaceEx(
				windows.StringToUTF16Ptr(p.MountPoint),
				&freeBytesAvailable,
				&totalNumberOfBytes,
				&totalNumberOfFreeBytes,
			)
			if err == nil {
				totalUsedBytes += (totalNumberOfBytes - totalNumberOfFreeBytes)
			}
		}

		fyne.DoAndWait(func() {
			sizeLabel.SetText(fmt.Sprintf("%s / %s", formatBytes(totalUsedBytes), formatBytes(totalPartitionSize)))
		})

		var walkErr error
	outer:
		for i, p := range partitions {
			if len(partitions) > 1 {
				fyne.DoAndWait(func() {
					partitionsLabel.SetText(fmt.Sprintf("Partition %d / %d", i+1, len(partitions)))
				})
			}
			walkErr = filepath.Walk(p.MountPoint+"/", func(path string, info os.FileInfo, err error) error {
				if err != nil {
					if os.IsPermission(err) {
						return nil
					}
					return err
				}

				select {
				case <-cancelChan:
					return errors.New("operation cancelled")
				case <-pauseChan:
					select {
					case <-cancelChan:
						return errors.New("operation cancelled")
					case <-pauseChan:
					}
				default:
				}

				if !info.IsDir() {
					time.Sleep(10 * time.Millisecond)
					accumulatedSize += uint64(info.Size())
					fyne.DoAndWait(func() {
						if totalPartitionSize > 0 {
							prg.SetValue(float64(accumulatedSize) / float64(totalPartitionSize))
						}
						path, _ = shortenPath(path)
						textArea.SetText(path)
					})
				}
				return nil
			})

			if walkErr != nil {
				break outer
			}
		}

		fyne.DoAndWait(func() {
			if walkErr != nil && walkErr.Error() == "operation cancelled" {
				dialog.ShowInformation("Cancelled", "Wipe operation was cancelled.", *window)
			} else if walkErr != nil {
				dialog.ShowError(walkErr, *window)
			} else {
				prg.SetValue(1.0)
				sizeLabel.SetText(fmt.Sprintf("%s / %s", formatBytes(totalPartitionSize), formatBytes(totalPartitionSize)))
				dialog.ShowInformation("Success", "Wipe complete!", *window)
				app.SendNotification(fyne.NewNotification("Success", "Wipe Complete"))
			}
		})
	}()

	progressWindow.Show()
	return true, nil
}

func Wipr(app fyne.App, window *fyne.Window, box *fyne.Container, data Data) (success bool, err error) {
	if data.Mode != "By Partitions" && data.Mode != "By Disk Drive" {
		return false, errors.New("invalid mode")
	}
	switch data.Mode {
	case "By Partitions":
		partition := partitionMap[data.Path]
		if partition == nil {
			return false, errors.New("invalid partition")
		}
		return wipePartitions(app, window, []*ghw.Partition{partition})
	case "By Disk Drive":
		drive := driveMap[data.Path]
		if drive == nil {
			return false, errors.New("invalid drive")
		}
		return wipePartitions(app, window, drive.Partitions)
	}
	return false, errors.New("invalid option")
}
