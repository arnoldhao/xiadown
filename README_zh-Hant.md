<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="下蛋 / XiaDown 圖示" />
  <h1>下蛋 / XiaDown</h1>
  <p><strong>一款支援影片下載的資源庫管理應用</strong></p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="最新版本" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="授權條款" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="支援平台" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="技術架構" />
  </p>
  <p>
    <a href="https://xiadown.app/">官方網站</a> ·
    <a href="https://xiadown.app/docs/">使用說明</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">發佈頁</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">回報問題</a> ·
    <a href="https://ko-fi.com/arnoldhao">贊助</a>
  </p>
  <p>
    <a href="./README.md">简体中文</a> ·
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
  <a href="https://xiadown.app/docs/library/">
    <img src="./images/library.webp" alt="下蛋資源庫介面" width="92%" />
  </a>
  <br />
  <strong>資源庫</strong>
</p>

## 專案簡介

下蛋是一款本機優先的資源庫管理應用，支援影片下載與嗅探下載。下蛋同時也是 YouTube、YouTube Music 與 RSS 用戶端，瀏覽內容時可一鍵下載所需媒體。

## 核心功能

- 🗂️ **[資源庫](https://xiadown.app/docs/library/)** — 影片下載、轉檔與資源庫管理。
- 🔎 **[嗅探](https://xiadown.app/docs/sniff/)** — 網頁媒體嗅探與下載。
- 🎵 **[音樂](https://xiadown.app/docs/music/)** — YouTube Music 瀏覽與下載、本機音樂播放。
- ▶️ **[YouTube](https://xiadown.app/docs/youtube/)** — 影片瀏覽、播放與下載。
- 📡 **[RSS](https://xiadown.app/docs/rss/)** — 內容訂閱、閱讀與媒體下載。

## 行動版

📱 iPhone 與 iPad 版正在開發中。

## 產品介面

<table>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/sniff/">
        <img src="./images/sniff-desk.webp" alt="下蛋嗅探介面" width="100%" />
      </a>
      <br />
      <strong>嗅探</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/rss/">
        <img src="./images/rss.webp" alt="下蛋 RSS 內容訂閱介面" width="100%" />
      </a>
      <br />
      <strong>RSS</strong>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/music/">
        <img src="./images/youtube-music.webp" alt="下蛋音樂介面" width="100%" />
      </a>
      <br />
      <strong>音樂</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/youtube/">
        <img src="./images/youtube-live.webp" alt="下蛋 YouTube 影片瀏覽介面" width="100%" />
      </a>
      <br />
      <strong>YouTube</strong>
    </td>
  </tr>
</table>

## 安裝

### Homebrew

macOS 可透過 Homebrew cask 安裝：

```bash
brew install --cask arnoldhao/tap/xiadown
```

### 下載安裝套件

| 平台 | 架構 | 形式 | 下載 |
| --- | --- | --- | --- |
| macOS | Apple 晶片 | DMG | [點此下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [點此下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | 安裝版 | [點此下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | 可攜版 | [點此下載](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

> macOS 版本需要 macOS 14（Sonoma）或更新版本；Windows 版本需要 Windows 10 或更新版本。

首次啟動時，系統會引導你完成語言、外觀、網路及執行階段相依套件的設定。詳細步驟請參閱[安裝與首次啟動](https://xiadown.app/docs/start/install/)。

## 本機開發

開發環境需要 Go 1.25.12、Node.js 24、Bun 1.3.5 與 Wails 3 alpha2.117：

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
wails3 task dev
```

其他建置與檢查工作請參閱 [Taskfile.yml](./Taskfile.yml)。

## 免責聲明

- 下蛋僅用於管理媒體及儲存使用者有權存取和使用的內容。
- 使用者應自行確認下載、儲存、轉換與使用相關內容符合所在地法律、權利人授權及目標平台服務條款。
- 請勿使用下蛋處理侵權、未授權、需付費、受限制、涉及隱私或其他違法內容。
- 因使用下蛋而衍生的著作權、平台規範、帳號、網路及其他相關責任，均由使用者自行承擔。

## 致謝

下蛋使用了 [Go](https://go.dev/)、[Wails](https://v3alpha.wails.io/)、[React](https://react.dev/)、[yt-dlp](https://github.com/yt-dlp/yt-dlp)、[FFmpeg](https://ffmpeg.org/) 與 [SQLite](https://www.sqlite.org/) 等開源專案。相依套件與授權資訊請參閱 [THIRD_PARTY_NOTICES.txt](./frontend/public/THIRD_PARTY_NOTICES.txt)。

## 協作

- 本專案目前不接受 PR，歡迎透過 [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) 回報問題或提出建議。
- 本儲存庫採用 `Apache-2.0` 授權條款，詳見 [LICENSE](./LICENSE)。

## 聯絡方式

- 官方網站：<https://xiadown.app/>
- 使用說明：<https://xiadown.app/docs/>
- 電子郵件：<xunruhao@gmail.com>
