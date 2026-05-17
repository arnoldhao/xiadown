<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="下蛋 / XiaDown 圖示" />
  <h1>下蛋 / XiaDown</h1>
  <p><strong>一款支援線上音樂的影片下載工具</strong></p>
  <p>Listen Keep, Make it Yours · 隨你，聽存隨心</p>
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
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="最新版本" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="授權條款" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="支援平台" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="技術棧" />
  </p>
</div>

## 專案簡介

下蛋是一款線上音樂播放器，也是一款影片下載工具。

它是為內容創作者打造的：需要素材時，提供基於 YT-DLP 的強大下載能力；需要工作時，在背景提供線上音樂播放能力。同時依託寵物與自訂外觀，讓軟體保持簡約，也不顯得乏味。

## 主要能力

- 🎧 **線上音樂播放器**：為 YouTube Lo-Fi 電台與 YouTube Music 設計的桌面播放器，支援帳號登入、歌曲/藝人/歌單搜尋、播放佇列、歌詞與封面顯示，也支援加入自訂線上 Lo-Fi 電台；遇到想長期保留的線上曲目，可繼續下載到本機資源庫。
- 📥 **影片與音訊下載**：基於 YT-DLP，支援上千個線上影片網站的素材下載；貼上連結即可儲存影片、音訊、字幕和封面，下載後可繼續轉檔並在資源庫裡統一管理。
- 🧩 **個人化使用空間**：內建精心設計的佈景主題包、強調色、外觀模式與側邊欄樣式，並提供完整的 Codex Pets 支援；依賴與軟體更新會自動維護，適合長期作為自己的媒體工具使用。

## 產品介面

<p align="center">
  <img src="./images/download.png" alt="下蛋下載任務介面" width="88%" />
</p>

<p align="center">
  <img src="./images/listen.png" alt="下蛋 Listen 線上音樂播放介面" width="88%" />
</p>

<p align="center">
  <img src="./images/library.png" alt="下蛋資源庫介面" width="88%" />
</p>

## 快速開始

### 下載安裝

可直接下載最新安裝包；歷史版本見 [GitHub 發布頁](https://github.com/arnoldhao/xiadown/releases)。

| 平台 | 架構 | 形式 | 下載 |
| --- | --- | --- | --- |
| macOS | Apple 晶片 | 壓縮檔 | [點擊下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | 壓縮檔 | [點擊下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | 安裝版 | [點擊下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | 可攜版 | [點擊下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### 首次開啟

1. `macOS`：解壓縮後將 `XiaDown.app` 移動到「應用程式」目錄。若系統提示「無法打開」或「已損毀」，請在終端機執行 `sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app`。
2. `Windows`：安裝版直接執行 `.exe`；可攜版解壓縮後直接啟動。若首次啟動出現 SmartScreen，選擇「更多資訊 -> 仍要執行」。
3. 首次啟動會進入歡迎引導，完成語言、佈景主題、代理和依賴安裝後即可進入主介面。主要流程都集中在歡迎引導和介面內。

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

- 官網：<https://xiadown.dreamapp.cc/>
- 信箱：<xunruhao@gmail.com>
