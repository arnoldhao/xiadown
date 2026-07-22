<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDown icon" />
  <h1>XiaDown</h1>
  <p><strong>A media library manager with video download support</strong></p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Latest version" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="License" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Supported platforms" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Technology stack" />
  </p>
  <p>
    <a href="https://xiadown.app/">Website</a> ·
    <a href="https://xiadown.app/docs/">Documentation</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Releases</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Report an issue</a> ·
    <a href="https://ko-fi.com/arnoldhao">Sponsor</a>
  </p>
  <p>
    <a href="./README.md">简体中文</a> ·
    <a href="./README_zh-Hant.md">繁體中文</a> ·
    <strong>English</strong> ·
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
    <img src="./images/library.webp" alt="XiaDown Library" width="92%" />
  </a>
  <br />
  <strong>Library</strong>
</p>

## Overview

XiaDown is a local-first media library manager that supports video downloads and media sniffing. It also serves as a YouTube, YouTube Music, and RSS client. You can download media with one click as you browse.

## Core capabilities

- 🗂️ **[Library](https://xiadown.app/docs/library/)** — Video downloads, transcoding, and media library management.
- 🔎 **[Sniff](https://xiadown.app/docs/sniff/)** — Detect and download media from web pages.
- 🎵 **[Music](https://xiadown.app/docs/music/)** — Browse and download music from YouTube Music; play local music.
- ▶️ **[YouTube](https://xiadown.app/docs/youtube/)** — Browse, play, and download videos.
- 📡 **[RSS](https://xiadown.app/docs/rss/)** — Subscribe to feeds, read posts, and download media.

## Mobile

📱 The iPhone and iPad app is in development.

## Product interface

<table>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/sniff/">
        <img src="./images/sniff-desk.webp" alt="XiaDown Sniff" width="100%" />
      </a>
      <br />
      <strong>Sniff</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/rss/">
        <img src="./images/rss.webp" alt="XiaDown RSS" width="100%" />
      </a>
      <br />
      <strong>RSS</strong>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/music/">
        <img src="./images/youtube-music.webp" alt="XiaDown Music" width="100%" />
      </a>
      <br />
      <strong>Music</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/youtube/">
        <img src="./images/youtube-live.webp" alt="XiaDown YouTube" width="100%" />
      </a>
      <br />
      <strong>YouTube</strong>
    </td>
  </tr>
</table>

## Installation

### Homebrew

Install the macOS app with the Homebrew cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Download a package

| Platform | Architecture | Package | Download |
| --- | --- | --- | --- |
| macOS | Apple silicon | DMG | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Installer | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portable | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

> The macOS build requires macOS 14 (Sonoma) or later. The Windows build requires Windows 10 or later.

On first launch, XiaDown guides you through language, appearance, network, and runtime dependency settings. See [Installation and first launch](https://xiadown.app/docs/start/install/) for details.

## Local development

Development requires Go 1.25.12, Node.js 24, Bun 1.3.5, and Wails 3 alpha2.117:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
wails3 task dev
```

See [Taskfile.yml](./Taskfile.yml) for other build and validation tasks.

## Disclaimer

- XiaDown is intended only for managing media and saving content that you are authorized to access and use.
- You are responsible for ensuring that downloading, saving, converting, and using content complies with local laws, rights-holder permissions, and the target platform's terms of service.
- Do not use XiaDown to process infringing, unauthorized, paywalled, private, or otherwise unlawful content.
- You assume responsibility for any copyright, platform-policy, account, network, or other issues arising from your use of XiaDown.

## Acknowledgements

XiaDown uses open-source projects including [Go](https://go.dev/), [Wails](https://v3alpha.wails.io/), [React](https://react.dev/), [yt-dlp](https://github.com/yt-dlp/yt-dlp), [FFmpeg](https://ffmpeg.org/), and [SQLite](https://www.sqlite.org/). See [THIRD_PARTY_NOTICES.txt](./frontend/public/THIRD_PARTY_NOTICES.txt) for dependency and license information.

## Contributing

- The project is not accepting pull requests at this time. Please use [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) to report problems or share suggestions.
- This repository is licensed under `Apache-2.0`. See [LICENSE](./LICENSE).

## Contact

- Website: <https://xiadown.app/>
- Documentation: <https://xiadown.app/docs/>
- Email: <xunruhao@gmail.com>
