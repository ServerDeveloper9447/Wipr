//go:generate fyne bundle -o bundled.go Icon.png
//go:generate fyne bundle -o bundled.go -append Small_Icon.png
//go:generate fyne bundle -o bundled.go -append Icon.ico
package main

import (
	"errors"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/jaypipes/ghw"
	"github.com/zalando/go-keyring"
)

type Config struct {
	MinimizeOnClose bool
	EnterpriseMode  bool
	PassKey         string
}

func ternary[T any](cond bool, iftrue T, iffalse T) T {
	if cond {
		return iftrue
	}
	return iffalse
}

const (
	WIDTH  = 700
	HEIGHT = 400
)

var (
	isWiping       = false
	config         Config
	driveMap       = make(map[string]*ghw.Disk)
	partitionMap   = make(map[string]*ghw.Partition)
)

func GetKey() string {
	secret, err := keyring.Get("Wipr_verify", "Wipr_user")
	if err != nil {
		return secret
	}
	return ""
}

func SetKey(s string) {
	keyring.Set("Wipr_verify", "Wipr_user", s)
}

func DeleteKey() {
	keyring.Delete("Wipr_verify", "Wipr_user")
}

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

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func shortenPath(path string) (string, error) {
	cleanPath := filepath.ToSlash(path)

	dir, file := filepath.Split(cleanPath)

	if file == "" {
		return "", fmt.Errorf("invalid path: no file name found")
	}

	dirParts := strings.Split(strings.TrimSuffix(dir, "/"), "/")

	var shortDir []string
	if len(dirParts) > 3 {
		shortDir = append(shortDir, dirParts[0], "...")

		for i := len(dirParts) - 2; i < len(dirParts); i++ {
			folderName := dirParts[i]
			if len(folderName) > 10 {
				shortDir = append(shortDir, folderName[:5]+"[...]"+folderName[len(folderName)-5:])
			} else {
				shortDir = append(shortDir, folderName)
			}
		}
	} else {
		shortDir = dirParts
	}

	shortenedDir := strings.Join(shortDir, "/") + "/"

	ext := filepath.Ext(file)
	name := strings.TrimSuffix(file, ext)
	var shortFile string
	if len(name) > 10 {
		shortFile = name[:5] + "[...]" + name[len(name)-5:] + ext
	} else {
		shortFile = file
	}

	return shortenedDir + shortFile, nil
}

func init() {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		fmt.Println("Unsupported OS")
		return
	}
	setup_creds()
}

func main() {
	isElevated := ElevateOnLaunch()
	if !isElevated {
		os.Exit(0)
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
				SetKey(config.PassKey)
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
					DeleteKey()
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
			infoWindow := wipr.NewWindow("Wipr Info")
			infoWindow.Resize(fyne.NewSize(400, 300))
			infoWindow.SetFixedSize(true)
			logo := canvas.NewImageFromResource(resourceSmallIconPng)
			logo.FillMode = canvas.ImageFillStretch
			logo.SetMinSize(fyne.NewSquareSize(100))
			logo.Resize(fyne.NewSquareSize(100))
			infoTxt := widget.NewLabelWithStyle("Wipr is a data destruction tool made by US-BEE. \nData destroyed by this software due to user's fault is not the developers' responsibility.", fyne.TextAlignCenter, fyne.TextStyle{
				Italic: true,
			})
			infoTxt.Wrapping = fyne.TextWrapWord
			url, _ := url.Parse("https://wipr.vercel.app")
			box := container.New(
				NewCustomPaddedBoxLayout(15, 15),
				container.NewVBox(
					container.NewCenter(logo),
					widget.NewLabelWithStyle("Wipr", fyne.TextAlignCenter, fyne.TextStyle{Bold: true, Monospace: true}),
					infoTxt,
					layout.NewSpacer(),
					container.NewCenter(container.NewHBox(
						widget.NewLabelWithStyle("Licensed under the ", fyne.TextAlignTrailing, fyne.TextStyle{
							Italic: true,
						}),
						widget.NewLabelWithStyle("MIT License", fyne.TextAlignLeading, fyne.TextStyle{
							Bold:   true,
							Italic: true,
						}),
					)),
					container.NewCenter(container.NewHBox(
						widget.NewLabelWithStyle("Visit our ", fyne.TextAlignTrailing, fyne.TextStyle{
							Bold: true,
						}),
						widget.NewHyperlinkWithStyle("Website", url, fyne.TextAlignLeading, fyne.TextStyle{
							Bold: true,
						}),
					)),
				),
			)
			infoWindow.SetContent(box)
			infoWindow.CenterOnScreen()
			infoWindow.RequestFocus()
			infoWindow.Show()
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
		switch typeOptions.Selected {
		case "By Partitions":
			partition := partitionMap[selectOptions.Selected]
			if partition == nil {
				err := errors.New("invalid partition")
				dialog.ShowError(err, window)
				fmt.Println(err)
				return
			}
			wipePartitions(wipr, &window, []*ghw.Partition{partition})
		case "By Disk Drive":
			drive := driveMap[selectOptions.Selected]
			if drive == nil {
				err := errors.New("invalid drive")
				dialog.ShowError(err, window)
				fmt.Println(err)
				return
			}
			wipePartitions(wipr, &window, drive.Partitions)
		default:
			err := errors.New("invalid mode")
			dialog.ShowError(err, window)
			fmt.Println(err)
			return
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

	wipr.Lifecycle().SetOnStarted(func() {
		setupSystray(wipr, window)
	})

	window.CenterOnScreen()
	window.SetContent(content)
	window.RequestFocus()
	window.ShowAndRun()
}
