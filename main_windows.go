//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"fyne.io/systray"
	"github.com/danieljoos/wincred"
	"github.com/jaypipes/ghw"
	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"
)

var (
	showWinSystray *systray.MenuItem
	quitWinSystray *systray.MenuItem
)

func setup_creds() {
	key, err := wincred.GetGenericCredential("Wipr/ServerKey")
	if err != nil {
		return
	}
	config.PassKey = string(key.CredentialBlob)
	config.EnterpriseMode = true
}

func getVolumeName(mountPoint string) string {
	query := fmt.Sprintf("SELECT VolumeName FROM Win32_LogicalDisk WHERE DeviceID='%s'", mountPoint)
	var vn []struct{
		VolumeName *string
	}
	wmi.Query(query, &vn)
	return *vn[0].VolumeName
}

func List_Partitions() []string {
	block, _ := ghw.Block()
	paritions := []string{}
	for _, d := range block.Disks {
		for _, p := range d.Partitions {
			paritions = append(paritions, fmt.Sprintf("%s %s (%s)", p.MountPoint, getVolumeName(p.MountPoint), d.Model))
			partitionMap[fmt.Sprintf("%s %s (%s)", p.MountPoint, getVolumeName(p.MountPoint), d.Model)] = p
		}
	}
	return paritions
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
				statusLabel.SetText(fmt.Sprintf("Wiping partition %d/%d (%s)...", i+1, len(partitions), p.MountPoint))
			})
			cmd := exec.Command("cmd", "/c", fmt.Sprintf("format %s /P:3 /V:Wipr /FS:NTFS /X /Y", p.MountPoint))
			if output, err := cmd.CombinedOutput(); err != nil {
				wipeErr = fmt.Errorf("format failed on %s: %v\n%s", p.MountPoint, err, string(output))
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

func overwrite3Pass(d *ghw.Disk) error {
	devicePath := "\\\\.\\" + d.Name
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

func ataSecureErase(d *ghw.Disk) error {
	command := fmt.Sprintf("Get-Disk -SerialNumber '%s' | Clear-Disk -RemoveData -RemoveOEM -Confirm:$false", d.SerialNumber)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Clear-Disk failed: %v\n%s", err, string(output))
	}
	return nil
}

func recreatePrimaryPart(d *ghw.Disk) error {
	fmt.Printf("Attempting Secure Erase on %s...\n", d.Model)
	err := ataSecureErase(d)
	if err != nil {
		fmt.Printf("Secure Erase failed: %v. Falling back to 3-pass overwrite.\n", err)
		if err := overwrite3Pass(d); err != nil {
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
		return fmt.Errorf("recreation failed: %v\n%s", err, string(output))
	}

	return nil
}

func ElevateOnLaunch() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer token.Close()
	var elevation uint32
	var retLen uint32
	err = windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &retLen)
	if err != nil {
		fmt.Println(err)
		return false
	}
	if elevation == 0 {
		shell32 := windows.NewLazyDLL("shell32.dll")
		procShellExecute := shell32.NewProc("ShellExecuteW")
		exe, _ := os.Executable()
		ret, _, err := procShellExecute.Call(
			0,
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("runas"))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(exe))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(""))),
			0,
			0,
		)
		if ret > 32 {
			return false
		}
		fmt.Println(err)
		user32 := windows.NewLazyDLL("user32.dll")
		procMessageBoxW := user32.NewProc("MessageBoxW")
		procMessageBoxW.Call(
			uintptr(0),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Wipr needs admin access to launch"))),
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Failed to launch Wipr"))),
			uintptr(0),
		)
		return false
	}
	return true
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
