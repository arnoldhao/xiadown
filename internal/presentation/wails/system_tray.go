package wails

import (
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/icons"

	"xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/settings"
	"xiadown/internal/domain/update"
	"xiadown/internal/presentation/i18n"
)

type trayActions interface {
	ToggleMiniPlayer() bool
	OpenMainWindow()
	OpenNewDownload()
	OpenSettings()
	ApplyMenuBarVisibility(value string)
	Quit()
	OpenUpdate()
}

type SystemTrayController struct {
	app             *application.App
	tray            *application.SystemTray
	icon            []byte
	actions         trayActions
	stateMu         sync.RWMutex
	miniPlayer      application.Window
	updateAvailable bool
	updateState     update.Info
}

func NewSystemTrayController(app *application.App, actions trayActions, icon []byte) *SystemTrayController {
	return &SystemTrayController{
		app:     app,
		icon:    icon,
		actions: actions,
	}
}

// AttachMiniPlayer binds the lazily-created player to the already-running tray.
// Wails permits replacing the attachment after the native tray has started.
func (controller *SystemTrayController) AttachMiniPlayer(window application.Window) {
	if controller == nil || window == nil {
		return
	}
	controller.stateMu.Lock()
	controller.miniPlayer = window
	tray := controller.tray
	controller.stateMu.Unlock()
	if tray != nil {
		tray.AttachWindow(window).WindowOffset(10)
	}
}

func (controller *SystemTrayController) ToggleMiniPlayer() bool {
	if controller == nil {
		return false
	}
	controller.stateMu.RLock()
	miniPlayer := controller.miniPlayer
	tray := controller.tray
	controller.stateMu.RUnlock()
	if tray == nil || miniPlayer == nil {
		return false
	}
	tray.ToggleWindow()
	applyTrayMiniPlayerWindowShape(miniPlayer)
	return true
}

func (controller *SystemTrayController) Update(current dto.Settings) {
	controller.ensureTray()
	tray := controller.traySnapshot()
	if tray == nil {
		return
	}

	lang, err := settings.ParseLanguage(current.Language)
	if err != nil {
		lang = settings.DefaultLanguage
	}
	strings := i18n.TrayMenu(lang)
	tray.SetTooltip(i18n.WindowTitles(lang).Main)
	visibilityLabel := strings.ShowInMenuBar
	if runtime.GOOS == "windows" {
		visibilityLabel = strings.ShowTrayIcon
	}

	menuBarVisibility := current.MenuBarVisibility
	if runtime.GOOS == "windows" && menuBarVisibility == settings.MenuBarVisibilityNever.String() {
		menuBarVisibility = settings.MenuBarVisibilityWhenRunning.String()
	}

	menu := controller.app.NewMenu()
	menu.Add(strings.NewDownload).OnClick(func(_ *application.Context) {
		if controller.actions != nil {
			controller.actions.OpenNewDownload()
		}
	})
	menu.Add(strings.OpenApp).OnClick(func(_ *application.Context) {
		if controller.actions != nil {
			controller.actions.OpenMainWindow()
		}
	})
	menu.AddSeparator()
	if controller.appendUpdateMenuItem(menu, strings) {
		menu.AddSeparator()
	}
	menu.Add(strings.Settings).OnClick(func(_ *application.Context) {
		if controller.actions != nil {
			controller.actions.OpenSettings()
		}
	})
	menu.AddSeparator()

	visibilityMenu := menu.AddSubmenu(visibilityLabel)
	addVisibility := func(value, label string) {
		visibilityMenu.AddRadio(label, menuBarVisibility == value).OnClick(func(_ *application.Context) {
			if controller.actions != nil {
				controller.actions.ApplyMenuBarVisibility(value)
			}
		})
	}
	addVisibility(settings.MenuBarVisibilityAlways.String(), strings.ShowAlways)
	addVisibility(settings.MenuBarVisibilityWhenRunning.String(), strings.ShowWhenRunning)
	if runtime.GOOS != "windows" {
		addVisibility(settings.MenuBarVisibilityNever.String(), strings.ShowNever)
	}

	menu.AddSeparator()
	menu.Add(strings.Quit).OnClick(func(_ *application.Context) {
		if controller.actions != nil {
			actions := controller.actions
			if runtime.GOOS == "windows" {
				go func() {
					time.Sleep(150 * time.Millisecond)
					actions.Quit()
				}()
				return
			}
			actions.Quit()
		}
	})

	tray.SetMenu(menu)

	if menuBarVisibility == settings.MenuBarVisibilityNever.String() {
		tray.Hide()
	} else {
		tray.Show()
	}
}

func (controller *SystemTrayController) SetUpdateAvailable(available bool, current dto.Settings) {
	controller.updateAvailable = available
	controller.Update(current)
}

func (controller *SystemTrayController) SetUpdateState(info update.Info, current dto.Settings) {
	controller.updateState = info
	controller.updateAvailable = info.IsUpdateAvailable() || info.Status == update.StatusChecking || info.Status == update.StatusInstalling
	controller.Update(current)
}

func (controller *SystemTrayController) appendUpdateMenuItem(menu *application.Menu, strings i18n.TrayMenuStrings) bool {
	state := controller.updateState

	if state.Status == update.StatusChecking {
		menu.Add(strings.CheckingForUpdate).SetEnabled(false)
		return true
	}

	if state.IsUpdateAvailable() || state.Status == update.StatusReadyToRestart || state.Status == update.StatusInstalling {
		menu.Add(strings.InstallUpdate).OnClick(func(_ *application.Context) {
			if controller.actions != nil {
				controller.actions.OpenUpdate()
			}
		})
		return true
	}

	if controller.updateAvailable {
		menu.Add(strings.InstallUpdate).OnClick(func(_ *application.Context) {
			if controller.actions != nil {
				controller.actions.OpenUpdate()
			}
		})
		return true
	}

	return false
}

func (controller *SystemTrayController) ensureTray() {
	controller.stateMu.Lock()
	if controller.tray != nil {
		controller.stateMu.Unlock()
		return
	}
	tray := controller.app.SystemTray.New()
	controller.tray = tray
	miniPlayer := controller.miniPlayer
	controller.stateMu.Unlock()
	tray.SetTooltip("XiaDown")

	if controller.icon != nil {
		if runtime.GOOS == "darwin" {
			tray.SetTemplateIcon(controller.icon)
		} else {
			tray.SetIcon(controller.icon)
			tray.SetDarkModeIcon(controller.icon)
		}
	} else if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icons.SystrayMacTemplate)
	} else {
		tray.SetIcon(icons.SystrayLight)
		tray.SetDarkModeIcon(icons.SystrayDark)
	}

	if miniPlayer != nil {
		tray.AttachWindow(miniPlayer).WindowOffset(10)
	}

	tray.OnClick(func() {
		if controller.actions != nil && controller.actions.ToggleMiniPlayer() {
			return
		}
		tray.OpenMenu()
	})
	tray.OnRightClick(func() {
		tray.OpenMenu()
	})
	tray.OnDoubleClick(func() {
		if controller.actions != nil {
			controller.actions.OpenMainWindow()
		}
	})
}

func (controller *SystemTrayController) traySnapshot() *application.SystemTray {
	if controller == nil {
		return nil
	}
	controller.stateMu.RLock()
	defer controller.stateMu.RUnlock()
	return controller.tray
}
