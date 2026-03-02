package main

import (
	"errors"
	"fmt"
	"image/color"
	"net/url"
	"os"
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

	"wipr/internal/assets"
	"wipr/internal/config"
	"wipr/internal/platform"
	"wipr/internal/storage"
	"wipr/internal/ui"
	"wipr/internal/utils"
)

const (
	WIDTH  = 700
	HEIGHT = 400
)

var (
	isWiping = false
	cfg      = config.Config{
		MinimizeOnClose: false,
		EnterpriseMode:  false,
		PassKey:         "",
		SafeMode:        false,
	}
	images = make(map[string]*fyne.StaticResource)
	icons  = make(map[string]*theme.ThemedResource)
)

const WEBSITE_URL = "https://wipr.vercel.app"

func init() {
	directory, _ := assets.FS.ReadDir("assets")
	for _, v := range directory {
		file, _ := assets.FS.ReadFile(fmt.Sprintf("assets/%s", v.Name()))
		resource := fyne.NewStaticResource(v.Name(), file)
		if strings.HasSuffix(v.Name(), ".svg") {
			themedRes := theme.NewThemedResource(resource)
			icons[v.Name()] = themedRes
			continue
		}
		images[v.Name()] = resource
	}
	_, exists := os.LookupEnv("WIPRSAFEMODE")
	if exists {
		cfg.SafeMode = true
	}
	platform.SetupCreds(func(key string) {
		cfg.PassKey = key
		cfg.EnterpriseMode = true
	})
}

func main() {
	isElevated := platform.ElevateOnLaunch()
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
		if cfg.MinimizeOnClose {
			window.Hide()
			return
		}
		var modal *widget.PopUp
		box := container.New(ui.NewCustomPaddedBoxLayout(15, 15), container.NewPadded(container.NewVBox(
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
	if !cfg.EnterpriseMode {
		verifyBtn.Hide()
	}
	toolbar := widget.NewToolbar(
		widget.NewToolbarSpacer(),
		widget.NewToolbarAction(icons["android.svg"], func() {
			dialog.NewConfirm("Android Mode", "Are you sure want to activate android mode?", func(b bool) {
				if b {
					ui.AndroidMode(wipr, window, WIDTH, HEIGHT, WEBSITE_URL)
				}
			}, window).Show()
		}),
		widget.NewToolbarAction(theme.SettingsIcon(), func() {
			var modal *widget.PopUp

			key := widget.NewEntry()
			key.Password = true
			key.OnChanged = func(s string) {
				s = strings.ReplaceAll(s, " ", "")
				key.SetText(s)
			}
			if cfg.EnterpriseMode {
				key.Text = cfg.PassKey
			}

			if cfg.EnterpriseMode {
				key.Enable()
			} else {
				key.Disable()
			}
			checkB := widget.NewCheck("Enterprise Mode", func(b bool) {
				if b {
					key.Enable()
					cfg.EnterpriseMode = true
				} else {
					key.Disable()
					cfg.EnterpriseMode = false
				}
			})
			checkB.Checked = cfg.EnterpriseMode
			mOC := widget.NewCheck("Minimize on close", func(b bool) {
				cfg.MinimizeOnClose = b
			})
			mOC.Checked = cfg.MinimizeOnClose
			btn := widget.NewButtonWithIcon("Save", theme.CheckButtonCheckedIcon(), func() {
				if cfg.EnterpriseMode {
					if key.Text == "" {
						dialog.ShowError(errors.New("please enter a key"), window)
						return
					} else {
						if len(key.Text) != 16 {
							dialog.ShowError(errors.New("key must be of length 16"), window)
							return
						}
						cfg.PassKey = key.Text
						verifyBtn.Show()
						config.SetKey(key.Text)
					}
				} else {
					cfg.PassKey = ""
					verifyBtn.Hide()
					config.DeleteKey()
				}
				modal.Hide()
			})
			btn.Importance = widget.HighImportance
			box := container.New(ui.NewCustomPaddedBoxLayout(5, 5),
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
			var modal *widget.PopUp
			logo := canvas.NewImageFromResource(images["Small_Icon.png"])
			logo.FillMode = canvas.ImageFillStretch
			logo.SetMinSize(fyne.NewSquareSize(100))
			logo.Resize(fyne.NewSquareSize(100))
			infoTxt := widget.NewLabelWithStyle("Wipr is a data destruction tool made by US-BEE. Data destroyed by this software due to user's fault is not the developers' responsibility.", fyne.TextAlignCenter, fyne.TextStyle{
				Italic: true,
			})
			infoTxt.Wrapping = fyne.TextWrapWord
			url, _ := url.Parse(WEBSITE_URL)
			box := container.New(
				ui.NewCustomPaddedBoxLayout(15, 15),
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
			border := container.NewBorder(widget.NewToolbar(
				widget.NewToolbarSpacer(),
				widget.NewToolbarAction(theme.WindowCloseIcon(), func() {
					if modal != nil {
						modal.Hide()
					}
				}),
			), nil, nil, nil, box)
			modal = widget.NewModalPopUp(border, window.Canvas())
			modal.Resize(fyne.NewSize(400, 300))
			modal.Show()
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
	drives := storage.List_Drives()
	partitions := storage.List_Partitions()
	var wipeBtn *widget.Button
	warningPrimaryPartition := canvas.NewText("WARNING: This is the partition where your OS is installed.", color.RGBA{200, 10, 10, 1})
	warningPrimaryPartition.Hide()
	selectOptions := widget.NewSelect(drives, func(s string) {
		if wipeBtn != nil {
			wipeBtn.Enable()
		}
		if strings.HasPrefix(s, "C:") || strings.HasPrefix(s, "sda1") {
			warningPrimaryPartition.Show()
		} else {
			warningPrimaryPartition.Hide()
		}
	})
	var box *fyne.Container
	typeOptions := widget.NewSelect([]string{"By Disk Drive", "By Partitions"}, func(s string) {
		selectOptions.ClearSelected()
		if wipeBtn != nil {
			wipeBtn.Disable()
		}
		selectOptions.SetOptions(utils.Ternary(s == "By Disk Drive", append(drives, "All Drives"), partitions))
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
	wipeBtn = widget.NewButtonWithIcon("Wipe", theme.DeleteIcon(), func() {
		isWiping = true
		switch typeOptions.Selected {
		case "By Partitions":
			partition := storage.PartitionMap[selectOptions.Selected]
			if partition == nil {
				err := errors.New("invalid partition")
				dialog.ShowError(err, window)
				fmt.Println(err)
				return
			}
			storage.WipePartitions(wipr, &window, []*ghw.Partition{partition}, func() {
				isWiping = true
				platform.DisableSystray()
			}, func() {
				isWiping = false
				platform.EnableSystray()
			})
		case "By Disk Drive":
			if selectOptions.Selected == "All Drives" {
				for _, v := range storage.DriveMap {
					go func() {
						fyne.Do(func() {
							storage.WipePartitions(wipr, &window, v.Partitions, func() {
								isWiping = true
								platform.DisableSystray()
							}, func() {
								isWiping = false
								platform.EnableSystray()
							})
						})
					}()
				}
				break
			}
			drive := storage.DriveMap[selectOptions.Selected]
			if drive == nil {
				err := errors.New("invalid drive")
				dialog.ShowError(err, window)
				fmt.Println(err)
				return
			}
			storage.WipePartitions(wipr, &window, drive.Partitions, func() {
				isWiping = true
				platform.DisableSystray()
			}, func() {
				isWiping = false
				platform.EnableSystray()
			})
			if err := storage.RecreatePrimaryPart(drive); err != nil {
				dialog.ShowError(err, window)
			}
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
		warningPrimaryPartition,
		verifyBtn,
		wipeBtn,
	)
	wipeBtn.Importance = widget.DangerImportance
	ctn := container.New(
		ui.NewCustomPaddedBoxLayout(15, 0),
		container.NewPadded(box),
	)

	boxWithBg := container.NewStack(bg, ctn)
	content := container.NewBorder(toolbar, btmToolbar, nil, nil, boxWithBg)

	wipr.Lifecycle().SetOnStarted(func() {
		platform.SetupSystray(wipr, window, images, func() bool { return isWiping })
	})
	window.SetContent(content)
	window.CenterOnScreen()
	window.RequestFocus()
	window.ShowAndRun()
}
