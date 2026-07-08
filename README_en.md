<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDown icon" />
  <h1>XiaDown</h1>
  <p><strong>A dual-engine video downloader with online music support.</strong></p>
  <p>Listen Keep, Make it Yours</p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Latest version" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="License" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Supported platforms" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Tech stack" />
  </p>
  <p>
    <a href="https://xiadown.app/">Website</a> ·
    <a href="https://xiadown.app/en/docs/">User Guide</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Releases</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Issues</a> ·
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
  <img src="./images/download.webp" alt="XiaDown download task view" width="92%" />
  <br />
  <sub>Download and transcoding tasks</sub>
</p>

## Overview

XiaDown is an online music player and a dual-engine video downloader.

It is a daily tool built for content creators: when you need material, use sniffing and YT-DLP to download it; when you need to focus, keep online music playing in the background; rich customization options help long-term use stay easy and fresh.

## Main Capabilities

### 📥 Download and Transcode

- **Sniffing downloads**: observes page videos, audio, live streams, manifests, images, subtitles, and API responses through CDP; suitable for sites such as TikTok, Douyin, Kuaishou, and Xiaohongshu that require a real browser session.
- **YT-DLP downloads**: paste a link to parse common platforms such as YouTube and Bilibili, save video, audio, subtitles, and covers, and use a signed-in identity to download content you are authorized to access.
- **Audio and video transcoding**: powered by FFmpeg, supports post-download transcoding and local file transcoding, with built-in presets such as H.264, H.265, VP9, MP3, AAC, Opus, FLAC, and WAV.

### 🗂️ Resource Management

- **Multi-view resource management**: task and file views unify downloads, transcodes, subtitles, covers, and imported files, with preview, details, batch selection, deletion, failed-task recovery, and stale-record cleanup.

### 🎧 Player

- **Local music playback**: automatically indexes library audio and supports queues, artwork, synced lyrics, East Asian romanized/pinyin lyrics, an equalizer, and spectrum visualizations.
- **YouTube Music**: search songs, artists, and playlists in a desktop-style experience, with home recommendations, playlist library, followed artists, liked music, playback queue, and lyrics.
- **YouTube Live**: create custom live groups and channels, view live status, play live radio, and open live video directly.

### 🔐 Safety and Isolation

- **Automatic dependency management**: installs, verifies, and updates YT-DLP, FFmpeg, Bun, and related tools automatically; tool paths are maintained by the app and do not pollute the system environment.
- **Credential and user isolation**: app session data comes from user-initiated sign-in and is stored independently with system encryption on macOS and Windows; connection settings stay separate from everyday browsing.

### 🎨 Freedom

- **Appearance customization**: supports theme packs, light/dark/auto modes, accent colors, fonts, font sizes, and sidebar styles; the built-in Codex Pets Gallery can import online and local Pets.

## Product Preview

<p align="center">
  <img src="./images/sniff-desk.webp" alt="XiaDown sniffing desk resource capture view" width="92%" />
  <br />
  <sub>Sniffing desk for capturing page resources</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="XiaDown YouTube Music playback view" width="92%" />
  <br />
  <sub>Desktop YouTube Music playback</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="XiaDown library view" width="92%" />
  <br />
  <sub>Unified library for downloaded and transcoded content</sub>
</p>

<details>
  <summary><strong>More Interface Screenshots</strong></summary>

  <p align="center">
    <img src="./images/youtube-live.webp" alt="XiaDown YouTube Live video view" width="92%" />
    <br />
    <sub>YouTube Live video viewing</sub>
  </p>

  <p align="center">
    <img src="./images/app_sessions.webp" alt="XiaDown app sessions and sign-in status view" width="92%" />
    <br />
    <sub>App sessions, sign-in verification, and account status management</sub>
  </p>

  <p align="center">
    <img src="./images/settings_download.webp" alt="XiaDown download settings and dependency tool management view" width="92%" />
    <br />
    <sub>Download directory, concurrency, and YT-DLP, FFmpeg, Bun management</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="XiaDown appearance settings view" width="92%" />
    <br />
    <sub>Themes, light/dark mode, accent colors, fonts, and font sizes</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="XiaDown Codex Pets Gallery view" width="92%" />
    <br />
    <sub>Codex Pets Gallery and local pet imports</sub>
  </p>
</details>

## Installation

### Homebrew

On macOS, install with Homebrew cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Download Installers

Download the latest package directly below. Older versions are available on [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Platform | Architecture | Package | Download |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Installer | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portable | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### First Launch

1. `macOS`: open the `.dmg`, drag `XiaDown.app` to the Applications folder, and open it.
2. `Windows`: run the `.exe` installer directly, or unzip the portable package and launch it. If SmartScreen appears on first launch, choose `More info -> Run anyway`.
3. XiaDown opens an onboarding flow for language, theme, proxy, and dependency setup. The main workflows are in the onboarding flow and UI.

### CDP Browser Support

Currently supported browsers:

| Mainstream | Privacy and Productivity | Specialty and Regional |
| --- | --- | --- |
| Chrome, Chromium, Edge | Brave, Vivaldi, Arc, Helium | Opera, Opera GX, Yandex Browser |

## Local Development

After preparing Go and Bun, install the Wails 3 CLI and start development mode:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.95
wails3 dev
```

## Disclaimer

- XiaDown is provided as a media management and download assistant, for learning, research, and saving content that you are authorized to access and use.
- You are responsible for ensuring any download, storage, conversion, or use of content is authorized by the rights holder and complies with applicable laws and the terms of the target site/platform.
- Do not use XiaDown to download, distribute, sell, or otherwise exploit infringing, unauthorized, paid/restricted, private, or otherwise unlawful content.
- Any copyright, platform policy, account, network, or other legal responsibility arising from your use of XiaDown is borne by you; the project maintainers are not responsible for user conduct or its consequences.

## Acknowledgements

XiaDown is built on top of excellent open-source projects. The desktop experience, media processing, local storage, browser connections, online music, and interface capabilities all depend on these foundations.

| Category | Homepage |
| --- | --- |
| Desktop Framework | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| Media Processing | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| Local Storage | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| Browser Connections | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| Frontend Experience | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## Collaboration

- The project is under active development and is not accepting pull requests for now. Feedback, bug reports, and usage scenarios are welcome through [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) or email.
- This repository is licensed under `Apache-2.0`. See [LICENSE](./LICENSE).

## Contact

- Website: <https://xiadown.app/>
- User Guide: <https://xiadown.app/en/docs/>
- Email: <xunruhao@gmail.com>
