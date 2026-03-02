//go:build linux && !android

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"fyne.io/systray"
	"github.com/jaypipes/ghw"
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

func overwrite3Pass(path string) error {
	cmd := exec.Command("shred", "-n", "3", "-z", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("shred failed on %s: %v\n%s", path, err, string(output))
	}
	return nil
}

func ataSecureErase(devicePath string) error {
	checkCmd := exec.Command("hdparm", "-I", devicePath)
	output, err := checkCmd.CombinedOutput()
	if err != nil {
		return err
	}
	if !strings.Contains(string(output), "supported") || strings.Contains(string(output), "frozen") {
		return errors.New("ATA Secure Erase not supported or drive is frozen")
	}

	setPass := exec.Command("hdparm", "--user-master", "u", "--security-set-pass", "wipr", devicePath)
	if output, err := setPass.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set security password: %v\n%s", err, string(output))
	}

	eraseCmd := exec.Command("hdparm", "--user-master", "u", "--security-erase", "wipr", devicePath)
	if output, err := eraseCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to execute security erase: %v\n%s", err, string(output))
	}
	return nil
}

func recreatePrimaryPart(d *ghw.Disk) error {
	devicePath := "/dev/" + d.Name

	fmt.Printf("Attempting ATA Secure Erase on %s...\n", devicePath)
	err := ataSecureErase(devicePath)
	if err != nil {
		fmt.Printf("ATA Secure Erase failed: %v. Falling back to 3-pass overwrite.\n", err)
		if err := overwrite3Pass(devicePath); err != nil {
			return err
		}
	}

	wipeCmd := exec.Command("wipefs", "--all", devicePath)
	output, err := wipeCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error wiping signatures: %s\n%s\n", err, string(output))
		return err
	}
	fmt.Println("Signatures wiped successfully.")

	fmt.Printf("Creating new GPT partition on %s...\n", devicePath)

	partitionLayout := "label: gpt\n,"

	sfdiskCmd := exec.Command("sfdisk", devicePath)
	sfdiskCmd.Stdin = strings.NewReader(partitionLayout)

	output, err = sfdiskCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running sfdisk: %s\n%s\n", err, string(output))
		return err
	}
	fmt.Println("Partition created successfully.")
	fmt.Printf("sfdisk output:\n%s\n", string(output))

	newPartitionPath := devicePath + "1"
	if strings.Contains(devicePath, "nvme") || strings.Contains(devicePath, "mmcblk") {
		newPartitionPath = devicePath + "p1"
	}
	fmt.Printf("Formatting %s with ext4...\n", newPartitionPath)

	mkfsCmd := exec.Command("mkfs.ext4", "-F", newPartitionPath)
	output, err = mkfsCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error formatting partition: %s\n%s\n", err, string(output))
		return err
	}

	fmt.Println("Partition formatted successfully.")
	fmt.Printf("mkfs output:\n%s\n", string(output))
	fmt.Println("\nDrive has been successfully re-partitioned and formatted.")
	return nil
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
	statusLabel := widget.NewLabel("Wiping partitions...")
	prg := widget.NewProgressBarInfinite()

	progressBox := container.NewVBox(
		layout.NewSpacer(),
		statusLabel,
		prg,
		layout.NewSpacer(),
	)
	progressWindow.SetContent(progressBox)
	progressWindow.Resize(fyne.NewSize(300, 150))
	progressWindow.CenterOnScreen()

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

		var wipeErr error
		for i, p := range partitions {
			fyne.DoAndWait(func() {
				statusLabel.SetText(fmt.Sprintf("Wiping partition %d/%d (%s)...", i+1, len(partitions), p.Name))
			})
			devicePath := "/dev/" + p.Name
			if err := overwrite3Pass(devicePath); err != nil {
				wipeErr = err
				break
			}
		}

		fyne.DoAndWait(func() {
			if wipeErr != nil {
				dialog.ShowError(wipeErr, *window)
			} else {
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
