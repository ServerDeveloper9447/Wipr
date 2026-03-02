//go:build linux && !android

package storage

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/jaypipes/ghw"
)

func List_Partitions() []string {
	block, _ := ghw.Block()
	paritions := []string{}
	for _, d := range block.Disks {
		for _, p := range d.Partitions {
			paritions = append(paritions, fmt.Sprintf("%s %s", p.Name, d.Model))
			PartitionMap[fmt.Sprintf("%s %s", p.FilesystemLabel, d.Model)] = p
		}
	}
	return paritions
}

func WipePartitions(app fyne.App, window *fyne.Window, partitions []*ghw.Partition, onWipeStart func(), onWipeEnd func()) (success bool, err error) {
	(*window).Hide()
	onWipeStart()

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
				onWipeEnd()
				(*window).Show()
				progressWindow.Close()
			})
		}()

		var wipeErr error
		for i, p := range partitions {
			fyne.DoAndWait(func() {
				statusLabel.SetText(fmt.Sprintf("Wiping partition %d/%d (%s)...", i+1, len(partitions), p.Name))
			})
			devicePath := "/dev/" + p.Name
			if err := Overwrite3Pass(devicePath); err != nil {
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

func Overwrite3Pass(path string) error {
	cmd := exec.Command("shred", "-n", "3", "-z", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("shred failed on %s: %v %s", path, err, string(output))
	}
	return nil
}

func AtaSecureErase(devicePath string) error {
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
		return fmt.Errorf("failed to set security password: %v %s", err, string(output))
	}

	eraseCmd := exec.Command("hdparm", "--user-master", "u", "--security-erase", "wipr", devicePath)
	if output, err := eraseCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to execute security erase: %v %s", err, string(output))
	}
	return nil
}

func RecreatePrimaryPart(d *ghw.Disk) error {
	devicePath := "/dev/" + d.Name

	fmt.Printf("Attempting ATA Secure Erase on %s...", devicePath)
	err := AtaSecureErase(devicePath)
	if err != nil {
		fmt.Printf("ATA Secure Erase failed: %v. Falling back to 3-pass overwrite.", err)
		if err := Overwrite3Pass(devicePath); err != nil {
			return err
		}
	}

	wipeCmd := exec.Command("wipefs", "--all", devicePath)
	output, err := wipeCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error wiping signatures: %s %s", err, string(output))
		return err
	}
	fmt.Println("Signatures wiped successfully.")

	fmt.Printf("Creating new GPT partition on %s...", devicePath)

	partitionLayout := "label: gpt,"

	sfdiskCmd := exec.Command("sfdisk", devicePath)
	sfdiskCmd.Stdin = strings.NewReader(partitionLayout)

	output, err = sfdiskCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running sfdisk: %s %s", err, string(output))
		return err
	}
	fmt.Println("Partition created successfully.")
	fmt.Printf("sfdisk output: %s", string(output))

	newPartitionPath := devicePath + "1"
	if strings.Contains(devicePath, "nvme") || strings.Contains(devicePath, "mmcblk") {
		newPartitionPath = devicePath + "p1"
	}
	fmt.Printf("Formatting %s with ext4...", newPartitionPath)

	mkfsCmd := exec.Command("mkfs.ext4", "-F", newPartitionPath)
	output, err = mkfsCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error formatting partition: %s %s", err, string(output))
		return err
	}

	fmt.Println("Partition formatted successfully.")
	fmt.Printf("mkfs output: %s", string(output))
	fmt.Println("Drive has been successfully re-partitioned and formatted.")
	return nil
}
