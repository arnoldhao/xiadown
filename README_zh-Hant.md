<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="下蛋 / XiaDown 圖示" />
  <h1>下蛋 / XiaDown</h1>
  <p><strong>一款支援線上音樂的雙引擎影片下載工具</strong></p>
  <p>Listen Keep, Make it Yours · 隨你，聽存隨心</p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="最新版本" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="授權條款" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="支援平台" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="技術棧" />
  </p>
  <p>
    <a href="https://xiadown.app/">官網</a> ·
    <a href="https://xiadown.app/zh-tw/docs/">使用說明</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">發布頁</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">回報問題</a> ·
    <a href="https://ko-fi.com/arnoldhao">贊助</a>
  </p>
  <p>
    <a href="./README.md">簡體中文</a> ·
    <strong>繁體中文</strong> ·
    <a href="./README_en.md">English</a> ·
    <a href="./README_ja-JP.md">日本語</a> ·
    <a href="./README_ko-KR.md">한국어</a> ·
    <a href="./README_es-419.md">Español (LatAm)</a> ·
    <a href="./README_pt-BR.md">Português (BR)</a> ·
    <a href="./README_id-ID.md">Bahasa Indonesia</a> ·
    <a href="./README_vi-VN.md">Tiếng Việt</a>
  </p>
</div>

<p align="center">
  <img src="./images/download.webp" alt="下蛋下載任務介面" width="92%" />
  <br />
  <sub>下載與轉檔任務</sub>
</p>

## 專案簡介

下蛋是一款線上音樂播放器，也是一款支援雙引擎下載的影片下載工具。

它是為內容創作者打造的日常工具：需要素材時，用嗅探和 YT-DLP 下載；需要專注時，在背景播放線上音樂；豐富的自訂選項，也讓長期使用保持輕鬆和新鮮。

## 主要能力

### 📥 下載與轉檔

- **嗅探下載**：基於 CDP 觀察網頁影片、音訊、直播流、清單、圖片、字幕和 API 回應；適合 TikTok、抖音、快手、小紅書等需要真實瀏覽器工作階段的站點。
- **YT-DLP 下載**：貼上連結即可解析 YouTube、嗶哩嗶哩等常用平台，儲存影片、音訊、字幕和封面，並支援使用已登入的身分下載有權存取的內容。
- **音影片轉檔**：基於 FFmpeg 支援下載後聯動轉檔和本機檔案轉檔，內建 H.264、H.265、VP9、MP3、AAC、Opus、FLAC、WAV 等常用預設。

### 🗂️ 資源管理

- **多視圖資源管理**：用任務視圖和檔案視圖統一管理下載、轉檔、字幕、封面和匯入檔案，支援預覽、詳情、批次選擇、刪除、失敗恢復和失效記錄清理。

### 🎧 播放器

- **本機音樂播放**：自動索引資源庫音訊，支援佇列、封面、同步歌詞、東亞羅馬音/拼音歌詞、等化器和頻譜視覺化。
- **YouTube Music**：以桌面化體驗搜尋歌曲、藝人和歌單，支援首頁推薦、歌單庫、關注藝人、喜歡的音樂、播放佇列與歌詞。
- **YouTube Live**：自訂直播分組與頻道，查看直播狀態，播放直播電台，也可以直接查看直播影片。

### 🔐 安全隔離

- **依賴自動管理**：自動安裝、校驗和升級 YT-DLP、FFmpeg、Bun 等工具，路徑由應用程式獨立維護，不污染系統環境。
- **憑證與使用者隔離**：應用工作階段資料來自使用者主動登入，並在 macOS 與 Windows 上使用系統加密能力獨立儲存；連線設定與日常瀏覽器隔離。

### 🎨 自由度

- **外觀自由定義**：支援佈景主題包、明暗/自動模式、強調色、字型、字號和側邊欄樣式；內建 Codex Pets Gallery，可匯入線上與本機 Pets。

## 產品介面

<p align="center">
  <img src="./images/sniff-desk.webp" alt="下蛋嗅探台資源捕獲介面" width="92%" />
  <br />
  <sub>嗅探台捕獲網頁資源</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="下蛋 YouTube Music 播放介面" width="92%" />
  <br />
  <sub>YouTube Music 桌面播放</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="下蛋資源庫介面" width="92%" />
  <br />
  <sub>資源庫統一管理下載與轉檔內容</sub>
</p>

<details>
  <summary><strong>更多介面截圖</strong></summary>

  <p align="center">
    <img src="./images/youtube-live.webp" alt="下蛋 YouTube Live 直播影片介面" width="92%" />
    <br />
    <sub>YouTube Live 直播影片查看</sub>
  </p>

  <p align="center">
    <img src="./images/app_sessions.webp" alt="下蛋應用工作階段與登入狀態介面" width="92%" />
    <br />
    <sub>應用工作階段、登入驗證與帳號狀態管理</sub>
  </p>

  <p align="center">
    <img src="./images/settings_download.webp" alt="下蛋下載設定與依賴工具管理介面" width="92%" />
    <br />
    <sub>下載目錄、並行數與 YT-DLP、FFmpeg、Bun 依賴管理</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="下蛋外觀設定介面" width="92%" />
    <br />
    <sub>佈景主題、明暗模式、強調色、字型與字號設定</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="下蛋 Codex Pets 寵物畫廊介面" width="92%" />
    <br />
    <sub>Codex Pets Gallery 與本機寵物匯入</sub>
  </p>
</details>

## 安裝方式

### Homebrew

macOS 可透過 Homebrew cask 安裝：

```bash
brew install --cask arnoldhao/tap/xiadown
```

### 下載安裝包

可直接下載最新安裝包；歷史版本見 [GitHub 發布頁](https://github.com/arnoldhao/xiadown/releases)。

| 平台 | 架構 | 形式 | 下載 |
| --- | --- | --- | --- |
| macOS | Apple 晶片 | DMG | [點擊下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [點擊下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | 安裝版 | [點擊下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | 可攜版 | [點擊下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### 首次開啟

1. `macOS`：開啟 `.dmg`，將 `XiaDown.app` 拖到「應用程式」目錄並開啟。
2. `Windows`：安裝版直接執行 `.exe`；可攜版解壓縮後直接啟動。若首次啟動出現 SmartScreen，選擇「更多資訊 -> 仍要執行」。
3. 首次啟動會進入歡迎引導，完成語言、佈景主題、代理和依賴安裝後即可進入主介面。主要流程都集中在歡迎引導和介面內。

### CDP 瀏覽器支援

目前支援的瀏覽器：

| 主流 | 隱私與效率 | 特色與區域 |
| --- | --- | --- |
| Chrome、Chromium、Edge | Brave、Vivaldi、Arc、Helium | Opera、Opera GX、Yandex Browser |

## 本機開發

準備好 Go 與 Bun 環境後，安裝 Wails 3 命令列並啟動開發模式：

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.95
wails3 dev
```

## 免責聲明

- 下蛋僅作為媒體管理與下載輔助工具提供，僅供學習、研究以及保存本人有權存取和使用的內容。
- 使用者應自行確認下載、保存、轉換和使用相關內容已取得權利人授權，且符合所在地區法律法規和目標網站/平台服務條款。
- 請勿使用下蛋下載、傳播、販售或以其他方式利用侵權、未授權、付費受限、隱私或其他違法違規內容。
- 因使用下蛋產生的著作權、平台規則、帳號、網路或其他法律責任由使用者自行承擔；專案維護者不對使用者行為及其後果負責。

## 致謝

下蛋建立在一系列優秀的開源專案之上。桌面體驗、媒體處理、本機儲存、瀏覽器連線、線上音樂與介面能力，都離不開這些依賴的支援。

| 分類 | 專案首頁 |
| --- | --- |
| 桌面框架 | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| 媒體處理 | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| 本機儲存 | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| 瀏覽器連線 | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| 前端體驗 | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## 協作

- 專案正在持續演進，當前暫不接受 PR，歡迎透過 [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) 或郵件回饋問題、分享建議與使用場景。
- 倉庫採用 `Apache-2.0` 授權條款，詳見 [LICENSE](./LICENSE)。

## 聯絡

- 官網：<https://xiadown.app/>
- 使用說明：<https://xiadown.app/zh-tw/docs/>
- 信箱：<xunruhao@gmail.com>
