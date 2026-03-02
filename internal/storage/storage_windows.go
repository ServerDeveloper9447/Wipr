//go:build windows

package storage

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/jaypipes/ghw"
	"github.com/yusufpapurcu/wmi"
)

func getVolumeName(mountPoint string) string {
	query := fmt.Sprintf("SELECT VolumeName FROM Win32_LogicalDisk WHERE DeviceID='%s'", mountPoint)
	var vn []struct {
		VolumeName *string
	}
	wmi.Query(query, &vn)
	if len(vn) == 0 || vn[0].VolumeName == nil {
		return ""
	}
	return *vn[0].VolumeName
}

func List_Partitions() []string {
	block, _ := ghw.Block()
	paritions := []string{}
	for _, d := range block.Disks {
		for _, p := range d.Partitions {
			paritions = append(paritions, fmt.Sprintf("%s %s (%s)", p.MountPoint, getVolumeName(p.MountPoint), d.Model))
			PartitionMap[fmt.Sprintf("%s %s (%s)", p.MountPoint, getVolumeName(p.MountPoint), d.Model)] = p
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
				statusLabel.SetText(fmt.Sprintf("Wiping partition %d/%d (%s)...", i+1, len(partitions), p.MountPoint))
			})
			cmd := exec.Command("cmd", "/c", fmt.Sprintf("format %s /P:3 /V:Wipr /FS:NTFS /X /Y", p.MountPoint))
			if output, err := cmd.CombinedOutput(); err != nil {
				wipeErr = fmt.Errorf("format failed on %s: %v %s", p.MountPoint, err, string(output))
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

func Overwrite3Pass(d *ghw.Disk) error {
	devicePath := "\\." + d.Name
	f, err := os.OpenFile(devicePath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	size := d.SizeBytes
	patterns := [][]byte{{0x00}, {0xFF}}

	for _, p := range patterns {
		pattern := make([]byte, 1024*1024)
		for i := range pattern {
			pattern[i] = p[0]
		}
		for written := uint64(0); written < size; {
			toWrite := uint64(len(pattern))
			if written+toWrite > size {
				toWrite = size - written
			}
			n, err := f.Write(pattern[:toWrite])
			if err != nil {
				return err
			}
			written += uint64(n)
		}
		f.Seek(0, 0)
	}

	// 3rd pass: Random
	for written := uint64(0); written < size; {
		pattern := make([]byte, 1024*1024)
		for i := range pattern {
			pattern[i] = byte(time.Now().UnixNano() % 256)
		}
		toWrite := uint64(len(pattern))
		if written+toWrite > size {
			toWrite = size - written
		}
		n, err := f.Write(pattern[:toWrite])
		if err != nil {
			return err
		}
		written += uint64(n)
	}

	f.Sync()
	return nil
}

func AtaSecureErase(d *ghw.Disk) error {
	command := fmt.Sprintf("Get-Disk -SerialNumber '%s' | Clear-Disk -RemoveData -RemoveOEM -Confirm:$false", d.SerialNumber)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Clear-Disk failed: %v %s", err, string(output))
	}
	return nil
}

func RecreatePrimaryPart(d *ghw.Disk) error {
	fmt.Printf("Attempting Secure Erase on %s...", d.Model)
	err := AtaSecureErase(d)
	if err != nil {
		fmt.Printf("Secure Erase failed: %v. Falling back to 3-pass overwrite.", err)
		if err := Overwrite3Pass(d); err != nil {
			return err
		}
	}

	command := fmt.Sprintf(
		"Get-Disk -SerialNumber '%s' | "+
			"Initialize-Disk -PartitionStyle GPT -PassThru | "+
			"New-Partition -UseMaximumSize -AssignDriveLetter | "+
			"Format-Volume -FileSystem NTFS -Confirm:$false",
		d.SerialNumber,
	)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", command)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("recreation failed: %v %s", err, string(output))
	}

	return nil
}
