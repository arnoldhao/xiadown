package wails

import (
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/icons"

	"xiadown/internal/application/settings/dto"
	"xiadown/internal/domain/settings"
	"xiadown/internal/domain/update"
	"xiadown/internal/presentation/i18n"
)

type trayActions interface {
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
	miniPlayer      application.Window
	updateAvailable bool
	updateState     update.Info
}

func NewSystemTrayController(app *application.App, actions trayActions, icon []byte, miniPlayer application.Window) *SystemTrayController {
	return &SystemTrayController{
		app:        app,
		icon:       icon,
		actions:    actions,
		miniPlayer: miniPlayer,
	}
}

func (controller *SystemTrayController) Update(current dto.Settings) {
	controller.ensureTray()
	if controller.tray == nil {
		return
	}

	lang, err := settings.ParseLanguage(current.Language)
	if err != nil {
		lang = settings.DefaultLanguage
	}
	strings := i18n.TrayMenu(lang)
	controller.tray.SetTooltip(i18n.WindowTitles(lang).Main)
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
			controller.actions.Quit()
		}
	})

	controller.tray.SetMenu(menu)

	if menuBarVisibility == settings.MenuBarVisibilityNever.String() {
		controller.tray.Hide()
	} else {
		controller.tray.Show()
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
				controller.actions.OpenSettings()
				controller.actions.OpenUpdate()
			}
		})
		return true
	}

	if controller.updateAvailable {
		menu.Add(strings.InstallUpdate).OnClick(func(_ *application.Context) {
			if controller.actions != nil {
				controller.actions.OpenSettings()
				controller.actions.OpenUpdate()
			}
		})
		return true
	}

	return false
}

func (controller *SystemTrayController) ensureTray() {
	if controller.tray != nil {
		return
	}
	controller.tray = controller.app.SystemTray.New()
	controller.tray.SetTooltip("XiaDown")

	if controller.icon != nil {
		if runtime.GOOS == "darwin" {
			controller.tray.SetTemplateIcon(controller.icon)
		} else {
			controller.tray.SetIcon(controller.icon)
			controller.tray.SetDarkModeIcon(controller.icon)
		}
	} else if runtime.GOOS == "darwin" {
		controller.tray.SetTemplateIcon(icons.SystrayMacTemplate)
	} else {
		controller.tray.SetIcon(icons.SystrayLight)
		controller.tray.SetDarkModeIcon(icons.SystrayDark)
	}

	if controller.miniPlayer != nil {
		controller.tray.AttachWindow(controller.miniPlayer).WindowOffset(10)
	}

	controller.tray.OnClick(func() {
		if controller.miniPlayer != nil {
			controller.tray.ToggleWindow()
			return
		}
		controller.tray.OpenMenu()
	})
	controller.tray.OnRightClick(func() {
		controller.tray.OpenMenu()
	})
	controller.tray.OnDoubleClick(func() {
		if controller.actions != nil {
			controller.actions.OpenMainWindow()
		}
	})
}
