//go:build windows

package main

import (
	"errors"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"fyne.io/systray"
	"github.com/danieljoos/wincred"
	"github.com/jaypipes/ghw"
	"golang.org/x/sys/windows"
)

type Data struct {
	Mode string
	Path string
}

func setup_creds() {
	key, err := wincred.GetGenericCredential("Wipr/ServerKey")
	if err != nil {
		return
	}
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

func Wipr_Win(app fyne.App, window *fyne.Window, box *fyne.Container, data Data) (success bool, err error) {
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

func Wipr() {
	isElevated := ElevateOnLaunch()
	if !isElevated {
		return
	}
	wipr := app.New()
	window := wipr.NewWindow("Wipr")
	window.Resize(fyne.NewSize(WIDTH, HEIGHT))
	window.SetTitle("Wipr")
	window.SetMaster()
	var modalHidden *bool
	t := true
	modalHidden = &t
	window.SetCloseIntercept(func() {
		if config.MinimizeOnClose {
			window.Hide()
			return
		}
		var modal *widget.PopUp
		box := container.New(NewCustomPaddedBoxLayout(15, 15), container.NewPadded(container.NewVBox(
			widget.NewLabel("Are you sure you want to quit?"),
			container.NewGridWithColumns(2, widget.NewButton("Yes", func() {
				wipr.Quit()
			}), widget.NewButton("Cancel", func() {
				modal.Hide()
				*modalHidden = true
			})))),
		)
		modal = widget.NewModalPopUp(box, window.Canvas())
		if *modalHidden {
			modal.Show()
			*modalHidden = false
		}
	})
	verifyBtn := widget.NewButtonWithIcon("Verify", theme.CheckButtonCheckedIcon(), func() {
		fmt.Println("Verifying methods...")
	})
	if !config.EnterpriseMode {
		verifyBtn.Hide()
	}
	toolbar := widget.NewToolbar(
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(theme.SettingsIcon(), func() {
			var modal *widget.PopUp

			key := widget.NewEntry()
			key.Password = true
			key.OnChanged = func(s string) {
				s = strings.ReplaceAll(s, " ", "")
				key.SetText(s)
			}
			if config.EnterpriseMode {
				key.Text = config.PassKey
			}

			btn := widget.NewButtonWithIcon("Connect", theme.CheckButtonCheckedIcon(), func() {
				config.EnterpriseMode = true
				if key.Text == "" {
					dialog.ShowError(errors.New("please enter key"), window)
					return
				}
				if len(key.Text) != 16 {
					dialog.ShowError(errors.New("key must be of length 16"), window)
					return
				}
				config.PassKey = key.Text
				keycred := wincred.NewGenericCredential("Wipr/ServerKey")
				keycred.CredentialBlob = []byte(config.PassKey)
				keycred.Persist = wincred.PersistLocalMachine
				keycred.Write()
				verifyBtn.Show()
				modal.Hide()
			})
			btn.Importance = widget.HighImportance
			if config.EnterpriseMode {
				key.Enable()
				btn.Enable()
			} else {
				key.Disable()
				btn.Disable()
			}
			checkB := widget.NewCheck("Enterprise Mode", func(b bool) {
				if b {
					key.Enable()
					btn.Enable()
				} else {
					key.Disable()
					btn.Disable()
					verifyBtn.Hide()
					config.EnterpriseMode = false
					keycred, _ := wincred.GetGenericCredential("Wipr/ServerKey")
					if keycred != nil {
						keycred.Delete()
					}
				}
			})
			checkB.Checked = config.EnterpriseMode
			mOC := widget.NewCheck("Minimize on close", func(b bool) {
				config.MinimizeOnClose = b
			})
			mOC.Checked = config.MinimizeOnClose
			box := container.New(NewCustomPaddedBoxLayout(5, 5),
				container.NewPadded(
					container.NewVBox(
						mOC,
						checkB,
						widget.NewLabel("Connection Key"),
						key,
						layout.NewSpacer(),
						container.NewGridWithColumns(2,
							btn,
							widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
								modal.Hide()
							}),
						),
					),
				),
			)
			modal = widget.NewModalPopUp(box, window.Canvas())
			modal.Resize(fyne.NewSize(WIDTH-300, HEIGHT-100))
			modal.Show()
		}),
		widget.NewToolbarAction(theme.HelpIcon(), func() {
			url, _ := url.Parse("https://wipr.vercel.app")
			wipr.OpenURL(url)
		}),
	)
	warningLabel := widget.NewLabel("Data deleted by Wipr is unrecoverable. Data destruction due to user error is not the responsibility of the developers.")
	warningLabel.Wrapping = fyne.TextWrapWord
	warningLabel.TextStyle = fyne.TextStyle{Bold: true, Underline: true}
	warningLabel.Alignment = fyne.TextAlignCenter
	btmToolbar := container.NewVBox(
		warningLabel,
		container.NewHBox(
			layout.NewSpacer(),
			widget.NewLabel("v"+wipr.Metadata().Version),
		))
	drives := List_Drives()
	partitions := List_Partitions()
	var wipeBtn *widget.Button
	selectOptions := widget.NewSelect(drives, func(s string) {
		if wipeBtn != nil {
			wipeBtn.Enable()
		}
	})
	typeOptions := widget.NewSelect([]string{"By Disk Drive", "By Partitions"}, func(s string) {
		selectOptions.ClearSelected()
		if wipeBtn != nil {
			wipeBtn.Disable()
		}
		selectOptions.SetOptions(ternary(s == "By Disk Drive", drives, partitions))
	})
	typeOptions.SetSelectedIndex(0)
	selectOptions.SetSelectedIndex(0)
	wiprText := canvas.NewText("Wipr", theme.Color(theme.ColorNameForeground))
	wiprText.TextSize = 20
	wiprText.Alignment = fyne.TextAlignCenter
	wiprText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(0, 30))

	bg := canvas.NewRectangle(color.Transparent)
	bg.SetMinSize(fyne.NewSize(WIDTH-100, HEIGHT))
	var box *fyne.Container
	wipeBtn = widget.NewButtonWithIcon("Wipe", theme.DeleteIcon(), func() {
		isWiping = true
		ok, err := Wipr_Win(wipr, &window, box, Data{Mode: typeOptions.Selected, Path: selectOptions.Selected})
		if !ok {
			dialog.ShowError(err, window)
			fmt.Println(err)
		}
	})
	box = container.NewVBox(wiprText,
		spacer,
		typeOptions,
		selectOptions,
		layout.NewSpacer(),
		verifyBtn,
		wipeBtn,
	)
	wipeBtn.Importance = widget.DangerImportance
	ctn := container.New(
		NewCustomPaddedBoxLayout(15, 0),
		container.NewPadded(box),
	)

	boxWithBg := container.NewStack(bg, ctn)
	content := container.NewBorder(toolbar, btmToolbar, nil, nil, boxWithBg)
	m := fyne.NewMenu("Wipr",
		fyne.NewMenuItem("Show", func() {
			if !isWiping {
				window.Show()
			}
		}),
		fyne.NewMenuItem("Quit", func() {
			wipr.Quit()
		}))
	if desk, ok := wipr.(desktop.App); ok {
		desk.SetSystemTrayMenu(m)
		wipr.Lifecycle().SetOnStarted(func() {
			systray.SetTooltip("Wipr v" + wipr.Metadata().Version)
		})
	}

	window.CenterOnScreen()
	window.SetContent(content)
	window.RequestFocus()
	window.ShowAndRun()
}
