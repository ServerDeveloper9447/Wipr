//go:build linux && !android

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"fyne.io/systray"
	"github.com/jaypipes/ghw"
	"golang.org/x/sys/unix"
	"r00t2.io/gosecret"
)

var (
	secretAttr = map[string]string{
		"appname": "com.usbee.wipr",
	}
	service *gosecret.Service
	showWinSystray *systray.MenuItem
	quitWinSystray *systray.MenuItem
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

func List_Partitions() []string {
	block, _ := ghw.Block()
	paritions := []string{}
	for _, d := range block.Disks {
		for _, p := range d.Partitions {
			paritions = append(paritions, fmt.Sprintf("%s %s", p.Name, d.Model))
			partitionMap[fmt.Sprintf("%s %s", p.FilesystemLabel, d.Model)] = p
		}
	}
	return paritions
}

func fillupDrive(app fyne.App, window *fyne.Window, disk *ghw.Disk) (success bool, err error) {
	return true, nil
}

func recreatePrimaryPart(disk string) {
	devicePath := "/dev/sdX"

	wipeCmd := exec.Command("wipefs", "--all", devicePath)
	output, err := wipeCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error wiping signatures: %s\n%s\n", err, string(output))
		return
	}
	fmt.Println("Signatures wiped successfully.")

	fmt.Printf("Creating new GPT partition on %s...\n", devicePath)

	partitionLayout := "label: gpt\n,"

	sfdiskCmd := exec.Command("sfdisk", devicePath)
	sfdiskCmd.Stdin = strings.NewReader(partitionLayout)

	output, err = sfdiskCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running sfdisk: %s\n%s\n", err, string(output))
		return
	}
	fmt.Println("Partition created successfully.")
	fmt.Printf("sfdisk output:\n%s\n", string(output))


	newPartitionPath := devicePath + "1"
	fmt.Printf("Formatting %s with ext4...\n", newPartitionPath)
	
	mkfsCmd := exec.Command("mkfs.ext4", "-F", newPartitionPath)
	output, err = mkfsCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error formatting partition: %s\n%s\n", err, string(output))
		return
	}

	fmt.Println("Partition formatted successfully.")
	fmt.Printf("mkfs output:\n%s\n", string(output))
	fmt.Println("\nDrive has been successfully re-partitioned and formatted.")
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

func setupSystray(wipr fyne.App, window fyne.Window) {
	systray.Register(func() {
		systray.SetIcon(images["Icon.ico"].StaticContent)
		systray.SetTemplateIcon(images["Icon.ico"].StaticContent, images["Icon.ico"].StaticContent)
		systray.SetTitle("Wipr v" + wipr.Metadata().Version)
		showWinSystray = systray.AddMenuItem("Show", "Show the Wipr window")
		quitWinSystray = systray.AddMenuItem("Quit", "Quit Wipr")
		go func() {
			for {
				select {
				case <-showWinSystray.ClickedCh:
					if !isWiping {
						fyne.Do(func() { window.Show() })
					}
				case <-quitWinSystray.ClickedCh:
					fyne.Do(func() { wipr.Quit() })
				}
			}
		}()
		systray.SetTooltip("Wipr v" + wipr.Metadata().Version)
	}, func() {})
}
