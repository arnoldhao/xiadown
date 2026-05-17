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
	case settings.LanguageJapanese:
		return MenuStrings{
			AppTitle:          "XiaDown",
			About:             "XiaDown について",
			Settings:          "設定…",
			CheckingForUpdate: "更新を確認中…",
			InstallUpdate:     "更新をインストール",
			Hide:              "XiaDownを隠す",
			HideOthers:        "ほかを隠す",
			ShowAll:           "すべて表示",
			Quit:              "XiaDownを終了",
			File:              "ファイル",
			Edit:              "編集",
			Undo:              "元に戻す",
			Redo:              "やり直し",
			Cut:               "カット",
			Copy:              "コピー",
			Paste:             "ペースト",
			Delete:            "削除",
			Close:             "ウィンドウを閉じる",
			SelectAll:         "すべて選択",
			Window:            "ウィンドウ",
			Minimize:          "最小化",
			Zoom:              "ズーム",
			FullScreen:        "フルスクリーン",
			BringAllToFront:   "すべてを前面に",
			Help:              "ヘルプ",
		}
	case settings.LanguageKorean:
		return MenuStrings{
			AppTitle:          "XiaDown",
			About:             "XiaDown 정보",
			Settings:          "설정…",
			CheckingForUpdate: "업데이트 확인 중…",
			InstallUpdate:     "업데이트 설치",
			Hide:              "XiaDown 숨기기",
			HideOthers:        "다른 항목 숨기기",
			ShowAll:           "모두 보이기",
			Quit:              "XiaDown 종료",
			File:              "파일",
			Edit:              "편집",
			Undo:              "실행 취소",
			Redo:              "다시 실행",
			Cut:               "오려두기",
			Copy:              "복사",
			Paste:             "붙여넣기",
			Delete:            "삭제",
			Close:             "창 닫기",
			SelectAll:         "모두 선택",
			Window:            "창",
			Minimize:          "최소화",
			Zoom:              "확대/축소",
			FullScreen:        "전체 화면",
			BringAllToFront:   "모두 앞으로 가져오기",
			Help:              "도움말",
		}
	case settings.LanguageSpanishLatinAmerica:
		return MenuStrings{
			AppTitle:          "XiaDown",
			About:             "Acerca de XiaDown",
			Settings:          "Configuración…",
			CheckingForUpdate: "Buscando actualización…",
			InstallUpdate:     "Instalar actualización",
			Hide:              "Ocultar XiaDown",
			HideOthers:        "Ocultar otros",
			ShowAll:           "Mostrar todo",
			Quit:              "Salir de XiaDown",
			File:              "Archivo",
			Edit:              "Editar",
			Undo:              "Deshacer",
			Redo:              "Rehacer",
			Cut:               "Cortar",
			Copy:              "Copiar",
			Paste:             "Pegar",
			Delete:            "Eliminar",
			Close:             "Cerrar ventana",
			SelectAll:         "Seleccionar todo",
			Window:            "Ventana",
			Minimize:          "Minimizar",
			Zoom:              "Zoom",
			FullScreen:        "Pantalla completa",
			BringAllToFront:   "Traer todo al frente",
			Help:              "Ayuda",
		}
	case settings.LanguagePortugueseBrazil:
		return MenuStrings{
			AppTitle:          "XiaDown",
			About:             "Sobre o XiaDown",
			Settings:          "Configurações…",
			CheckingForUpdate: "Verificando atualização…",
			InstallUpdate:     "Instalar atualização",
			Hide:              "Ocultar XiaDown",
			HideOthers:        "Ocultar outros",
			ShowAll:           "Mostrar tudo",
			Quit:              "Sair do XiaDown",
			File:              "Arquivo",
			Edit:              "Editar",
			Undo:              "Desfazer",
			Redo:              "Refazer",
			Cut:               "Cortar",
			Copy:              "Copiar",
			Paste:             "Colar",
			Delete:            "Excluir",
			Close:             "Fechar janela",
			SelectAll:         "Selecionar tudo",
			Window:            "Janela",
			Minimize:          "Minimizar",
			Zoom:              "Zoom",
			FullScreen:        "Tela cheia",
			BringAllToFront:   "Trazer tudo para frente",
			Help:              "Ajuda",
		}
	case settings.LanguageIndonesian:
		return MenuStrings{
			AppTitle:          "XiaDown",
			About:             "Tentang XiaDown",
			Settings:          "Pengaturan…",
			CheckingForUpdate: "Memeriksa pembaruan…",
			InstallUpdate:     "Instal pembaruan",
			Hide:              "Sembunyikan XiaDown",
			HideOthers:        "Sembunyikan lainnya",
			ShowAll:           "Tampilkan semua",
			Quit:              "Keluar dari XiaDown",
			File:              "File",
			Edit:              "Edit",
			Undo:              "Urungkan",
			Redo:              "Ulangi",
			Cut:               "Potong",
			Copy:              "Salin",
			Paste:             "Tempel",
			Delete:            "Hapus",
			Close:             "Tutup jendela",
			SelectAll:         "Pilih semua",
			Window:            "Jendela",
			Minimize:          "Perkecil",
			Zoom:              "Zoom",
			FullScreen:        "Layar penuh",
			BringAllToFront:   "Bawa semua ke depan",
			Help:              "Bantuan",
		}
	case settings.LanguageVietnamese:
		return MenuStrings{
			AppTitle:          "XiaDown",
			About:             "Giới thiệu XiaDown",
			Settings:          "Cài đặt…",
			CheckingForUpdate: "Đang kiểm tra cập nhật…",
			InstallUpdate:     "Cài bản cập nhật",
			Hide:              "Ẩn XiaDown",
			HideOthers:        "Ẩn ứng dụng khác",
			ShowAll:           "Hiển thị tất cả",
			Quit:              "Thoát XiaDown",
			File:              "Tệp",
			Edit:              "Sửa",
			Undo:              "Hoàn tác",
			Redo:              "Làm lại",
			Cut:               "Cắt",
			Copy:              "Sao chép",
			Paste:             "Dán",
			Delete:            "Xóa",
			Close:             "Đóng cửa sổ",
			SelectAll:         "Chọn tất cả",
			Window:            "Cửa sổ",
			Minimize:          "Thu nhỏ",
			Zoom:              "Zoom",
			FullScreen:        "Toàn màn hình",
			BringAllToFront:   "Đưa tất cả lên trước",
			Help:              "Trợ giúp",
		}
	case settings.LanguageChineseTraditional:
		return MenuStrings{
			AppTitle:          "下蛋",
			About:             "關於下蛋",
			Settings:          "偏好設定…",
			CheckingForUpdate: "正在檢查更新…",
			InstallUpdate:     "安裝更新",
			Hide:              "隱藏下蛋",
			HideOthers:        "隱藏其他",
			ShowAll:           "顯示全部",
			Quit:              "結束下蛋",
			File:              "檔案",
			Edit:              "編輯",
			Undo:              "復原",
			Redo:              "重做",
			Cut:               "剪下",
			Copy:              "複製",
			Paste:             "貼上",
			Delete:            "刪除",
			Close:             "關閉視窗",
			SelectAll:         "全選",
			Window:            "視窗",
			Minimize:          "最小化",
			Zoom:              "縮放",
			FullScreen:        "全螢幕",
			BringAllToFront:   "全部移到最前",
			Help:              "說明",
		}
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
	case settings.LanguageJapanese:
		return TrayMenuStrings{
			NewDownload:       "新規ダウンロード",
			OpenApp:           "XiaDownを開く",
			Settings:          "設定…",
			InstallUpdate:     "更新をインストール",
			CheckingForUpdate: "更新を確認中…",
			ShowInMenuBar:     "メニューバーに表示",
			ShowTrayIcon:      "トレイアイコンを表示",
			ShowAlways:        "常に表示",
			ShowWhenRunning:   "実行中のみ表示",
			ShowNever:         "表示しない",
			Quit:              "終了",
		}
	case settings.LanguageKorean:
		return TrayMenuStrings{
			NewDownload:       "새 다운로드",
			OpenApp:           "XiaDown 열기",
			Settings:          "설정…",
			InstallUpdate:     "업데이트 설치",
			CheckingForUpdate: "업데이트 확인 중…",
			ShowInMenuBar:     "메뉴 막대에 표시",
			ShowTrayIcon:      "트레이 아이콘 표시",
			ShowAlways:        "항상 표시",
			ShowWhenRunning:   "실행 중일 때 표시",
			ShowNever:         "표시 안 함",
			Quit:              "종료",
		}
	case settings.LanguageSpanishLatinAmerica:
		return TrayMenuStrings{
			NewDownload:       "Nueva descarga",
			OpenApp:           "Abrir XiaDown",
			Settings:          "Configuración…",
			InstallUpdate:     "Instalar actualización",
			CheckingForUpdate: "Buscando actualización…",
			ShowInMenuBar:     "Mostrar en barra de menú",
			ShowTrayIcon:      "Mostrar icono de bandeja",
			ShowAlways:        "Siempre",
			ShowWhenRunning:   "Cuando la app esté en ejecución",
			ShowNever:         "Nunca",
			Quit:              "Salir",
		}
	case settings.LanguagePortugueseBrazil:
		return TrayMenuStrings{
			NewDownload:       "Novo download",
			OpenApp:           "Abrir XiaDown",
			Settings:          "Configurações…",
			InstallUpdate:     "Instalar atualização",
			CheckingForUpdate: "Verificando atualização…",
			ShowInMenuBar:     "Mostrar na barra de menus",
			ShowTrayIcon:      "Mostrar ícone da bandeja",
			ShowAlways:        "Sempre",
			ShowWhenRunning:   "Quando o app estiver em execução",
			ShowNever:         "Nunca",
			Quit:              "Sair",
		}
	case settings.LanguageIndonesian:
		return TrayMenuStrings{
			NewDownload:       "Unduhan baru",
			OpenApp:           "Buka XiaDown",
			Settings:          "Pengaturan…",
			InstallUpdate:     "Instal pembaruan",
			CheckingForUpdate: "Memeriksa pembaruan…",
			ShowInMenuBar:     "Tampilkan di bilah menu",
			ShowTrayIcon:      "Tampilkan ikon tray",
			ShowAlways:        "Selalu",
			ShowWhenRunning:   "Saat aplikasi berjalan",
			ShowNever:         "Tidak pernah",
			Quit:              "Keluar",
		}
	case settings.LanguageVietnamese:
		return TrayMenuStrings{
			NewDownload:       "Tải xuống mới",
			OpenApp:           "Mở XiaDown",
			Settings:          "Cài đặt…",
			InstallUpdate:     "Cài bản cập nhật",
			CheckingForUpdate: "Đang kiểm tra cập nhật…",
			ShowInMenuBar:     "Hiển thị trên thanh menu",
			ShowTrayIcon:      "Hiển thị biểu tượng khay",
			ShowAlways:        "Luôn luôn",
			ShowWhenRunning:   "Khi ứng dụng đang chạy",
			ShowNever:         "Không bao giờ",
			Quit:              "Thoát",
		}
	case settings.LanguageChineseTraditional:
		return TrayMenuStrings{
			NewDownload:       "新增下載",
			OpenApp:           "開啟下蛋",
			Settings:          "偏好設定…",
			InstallUpdate:     "安裝更新",
			CheckingForUpdate: "正在檢查更新…",
			ShowInMenuBar:     "選單列顯示",
			ShowTrayIcon:      "系統匣圖示顯示",
			ShowAlways:        "一律顯示",
			ShowWhenRunning:   "執行時顯示",
			ShowNever:         "不顯示",
			Quit:              "結束",
		}
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
	case settings.LanguageJapanese:
		return WindowTitleStrings{
			Main:     "XiaDown",
			Settings: "設定",
		}
	case settings.LanguageKorean:
		return WindowTitleStrings{
			Main:     "XiaDown",
			Settings: "설정",
		}
	case settings.LanguageSpanishLatinAmerica:
		return WindowTitleStrings{
			Main:     "XiaDown",
			Settings: "Configuración",
		}
	case settings.LanguagePortugueseBrazil:
		return WindowTitleStrings{
			Main:     "XiaDown",
			Settings: "Configurações",
		}
	case settings.LanguageIndonesian:
		return WindowTitleStrings{
			Main:     "XiaDown",
			Settings: "Pengaturan",
		}
	case settings.LanguageVietnamese:
		return WindowTitleStrings{
			Main:     "XiaDown",
			Settings: "Cài đặt",
		}
	case settings.LanguageChineseTraditional:
		return WindowTitleStrings{
			Main:     "下蛋",
			Settings: "設定",
		}
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
