//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/jaypipes/ghw"
	"golang.org/x/sys/unix"
	"r00t2.io/gosecret"
)

type Data struct {
	Mode string
	Path string
}

var (
	secretAttr = map[string]string{
		"appname": "com.usbee.wipr",
	}
	service *gosecret.Service
)

func setup_creds() {
	var err error
	service, err = gosecret.NewService()
	if err != nil {
		fmt.Println(err)
		return
	}
	unlocked, _, err := service.SearchItems(secretAttr)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(unlocked) > 0 {
		item := unlocked[0]
		config.PassKey = string(item.Secret.Value)
	}
}

func setKey(s string) {
	secret := gosecret.NewSecret(
		service.Session,
		[]byte{},
		[]byte(s),
		"text/plain",
	)
	coll, _ := service.GetCollection("wipr_creds")
	if _, err := coll.CreateItem(
		"Server Key",
		secretAttr,
		secret,
		true,
	); err != nil {
		fmt.Println(err)
	}
}

func deleteKey() {
	unlocked, _, err := service.SearchItems(secretAttr)
	if err != nil {
		fmt.Println(err)
		return
	}
	item := unlocked[0]
	item.Delete()
}

func ElevateOnLaunch() bool {
	if os.Geteuid() != 0 {
		exe, _ := os.Executable()
		args := os.Args[1:]

		var envVars []string
		for _, envVar := range []string{"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "XDG_RUNTIME_DIR"} {
			if value := os.Getenv(envVar); value != "" {
				envVars = append(envVars, fmt.Sprintf("%s=%s", envVar, value))
			}
		}

		var cmd *exec.Cmd
		if len(envVars) > 0 {
			finalArgs := []string{"env"}
			finalArgs = append(finalArgs, envVars...)
			finalArgs = append(finalArgs, exe)
			finalArgs = append(finalArgs, args...)
			cmd = exec.Command("pkexec", finalArgs...)
		} else {
			cmd = exec.Command("pkexec", append([]string{exe}, args...)...)
		}

		if err := cmd.Run(); err != nil {
			fmt.Println(err)
			return false
		}
		return false
	}
	return true
}

func wipePartitions(app fyne.App, window *fyne.Window, partitions []*ghw.Partition) (success bool, err error) {
	isWiping = true
	(*window).Hide()
	if quitWinSystray != nil {
		quitWinSystray.Disable()
	}
	if showWinSystray != nil {
		showWinSystray.Disable()
	}

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
				if quitWinSystray != nil {
					quitWinSystray.Enable()
				}
				if showWinSystray != nil {
					showWinSystray.Enable()
				}
				progressWindow.Close()
			})
		}()

		var accumulatedSize uint64 = 0
		var totalPartitionSize uint64 = 0
		var totalUsedBytes uint64 = 0

		for _, p := range partitions {
			totalPartitionSize += p.SizeBytes
			var stat unix.Statfs_t
			err := unix.Statfs(p.MountPoint, &stat)
			if err == nil {
				totalNumberOfBytes := stat.Blocks * uint64(stat.Bsize)
				totalNumberOfFreeBytes := stat.Bfree * uint64(stat.Bsize)
				totalUsedBytes += (totalNumberOfBytes - totalNumberOfFreeBytes)
			}
		}

		fyne.DoAndWait(func() {
			sizeLabel.SetText(fmt.Sprintf("0 / %s", formatBytes(totalUsedBytes)))
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
						sizeLabel.SetText(fmt.Sprintf("%s / %s", formatBytes(accumulatedSize), formatBytes(totalUsedBytes)))
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
