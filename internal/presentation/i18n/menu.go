package i18n

import "xiadown/internal/domain/settings"

type MenuStrings struct {
	AppTitle          string
	About             string
	Settings          string
	CheckingForUpdate string
	InstallUpdate     string
	Hide              string
	HideOthers        string
	ShowAll           string
	Quit              string
	File              string
	Edit              string
	Undo              string
	Redo              string
	Cut               string
	Copy              string
	Paste             string
	Delete            string
	Close             string
	SelectAll         string
	Window            string
	Minimize          string
	Zoom              string
	FullScreen        string
	BringAllToFront   string
	Help              string
}

type TrayMenuStrings struct {
	NewDownload       string
	OpenApp           string
	Settings          string
	InstallUpdate     string
	CheckingForUpdate string
	ShowInMenuBar     string
	ShowTrayIcon      string
	ShowAlways        string
	ShowWhenRunning   string
	ShowNever         string
	Quit              string
}

type WindowTitleStrings struct {
	Main     string
	Settings string
}

func Menu(lang settings.Language) MenuStrings {
	switch lang {
	case settings.LanguageChineseSimplified:
		return MenuStrings{
			AppTitle:          "下蛋",
			About:             "关于下蛋",
			Settings:          "偏好设置…",
			CheckingForUpdate: "正在检查更新…",
			InstallUpdate:     "安装更新",
			Hide:              "隐藏下蛋",
			HideOthers:        "隐藏其他",
			ShowAll:           "显示全部",
			Quit:              "退出下蛋",
			File:              "文件",
			Edit:              "编辑",
			Undo:              "撤销",
			Redo:              "重做",
			Cut:               "剪切",
			Copy:              "复制",
			Paste:             "粘贴",
			Delete:            "删除",
			Close:             "关闭窗口",
			SelectAll:         "全选",
			Window:            "窗口",
			Minimize:          "最小化",
			Zoom:              "缩放",
			FullScreen:        "全屏",
			BringAllToFront:   "全部置前",
			Help:              "帮助",
		}
	default:
		return MenuStrings{
			AppTitle:          "XiaDown",
			About:             "About XiaDown",
			Settings:          "Settings…",
			CheckingForUpdate: "Checking for Update…",
			InstallUpdate:     "Install updates",
			Hide:              "Hide XiaDown",
			HideOthers:        "Hide Others",
			ShowAll:           "Show All",
			Quit:              "Quit XiaDown",
			File:              "File",
			Edit:              "Edit",
			Undo:              "Undo",
			Redo:              "Redo",
			Cut:               "Cut",
			Copy:              "Copy",
			Paste:             "Paste",
			Delete:            "Delete",
			Close:             "Close Window",
			SelectAll:         "Select All",
			Window:            "Window",
			Minimize:          "Minimize",
			Zoom:              "Zoom",
			FullScreen:        "Fullscreen",
			BringAllToFront:   "Bring All to Front",
			Help:              "Help",
		}
	}
}

func TrayMenu(lang settings.Language) TrayMenuStrings {
	switch lang {
	case settings.LanguageChineseSimplified:
		return TrayMenuStrings{
			NewDownload:       "新建下载",
			OpenApp:           "打开下蛋",
			Settings:          "偏好设置…",
			InstallUpdate:     "安装更新",
			CheckingForUpdate: "正在检查更新…",
			ShowInMenuBar:     "菜单栏显示",
			ShowTrayIcon:      "托盘图标显示",
			ShowAlways:        "总是显示",
			ShowWhenRunning:   "运行时显示",
			ShowNever:         "不显示",
			Quit:              "退出",
		}
	default:
		return TrayMenuStrings{
			NewDownload:       "New Download",
			OpenApp:           "Open XiaDown",
			Settings:          "Settings…",
			InstallUpdate:     "Install Updates",
			CheckingForUpdate: "Checking for Update…",
			ShowInMenuBar:     "Show in Menu Bar",
			ShowTrayIcon:      "Show Tray Icon",
			ShowAlways:        "Always",
			ShowWhenRunning:   "When App Is Running",
			ShowNever:         "Never",
			Quit:              "Quit",
		}
	}
}

func WindowTitles(lang settings.Language) WindowTitleStrings {
	switch lang {
	case settings.LanguageChineseSimplified:
		return WindowTitleStrings{
			Main:     "下蛋",
			Settings: "设置",
		}
	default:
		return WindowTitleStrings{
			Main:     "XiaDown",
			Settings: "Settings",
		}
	}
}
