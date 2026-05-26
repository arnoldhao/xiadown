<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDown icon" />
  <h1>XiaDown</h1>
  <p><strong>A dual-engine video download tool with online music support.</strong></p>
  <p>Listen Keep, Make it Yours</p>
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
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Latest version" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="License" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Supported platforms" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Tech stack" />
  </p>
</div>

## Overview

XiaDown is an online music player, and also a dual-engine video download tool.

It is built for content creators: when you need source material, it provides powerful download capabilities through browser sniffing and YT-DLP; when you need to work, it keeps online music playing in the background. With a library, transcoding, dependency management, account isolation, pets, and appearance customization, XiaDown can handle media material and also serve as your everyday desktop media tool.

## Main Capabilities

- 📥 **Dual-engine video downloads**: XiaDown includes a YT-DLP download engine and a CDP-based browser sniffing engine. Regular links can be parsed and downloaded directly, including with Cookies saved in connection profiles; for dynamically loaded pages, complex site structures, or resources that require a real browser session, sniffing mode can capture video, audio, subtitles, covers, and other media resources, then continue into transcoding and library management.
- 🎧 **Desktop music player**: automatically manages local audio from downloaded resources, supports YouTube Music search for songs, artists, and playlists, and supports YouTube Live radio playback with live video viewing. The player includes queues, artwork, synced lyrics, East Asian romanized/pinyin lyrics, an equalizer, and spectrum visualizations.
- 🧩 **A flexible, controlled workspace**: dependency tools are installed and upgraded automatically without polluting the system environment; accounts, Cookies, and browser Profiles are managed through isolated connection settings; themes, light/dark mode, accent colors, fonts, font sizes, and Codex Pets can all be customized.

## Core Capabilities

- **Sniffing downloads**: a self-developed CDP-based browser sniffing capability that can observe video, audio, live streams, manifests, images, subtitles, API responses, and other resources on a page. In a real browser environment where the user has explicitly signed in, it can identify and download resources from TikTok, Douyin, Kuaishou, Xiaohongshu, and similar sites, and it can link downloads directly into transcoding.
- **YT-DLP downloads**: integrates YT-DLP for downloading material from a wide range of online video sites, with stable support for common platforms such as YouTube and Bilibili. Paste a link to parse and save video, audio, subtitles, and covers; downloads can also use Cookies saved in connection profiles for content the user is authorized to access, then continue into transcoding and library management.
- **Audio and video transcoding**: powered by FFmpeg, with support for transcoding right after download or manually selecting local files. Built-in presets include H.264, H.265, VP9, MP3, AAC, Opus, FLAC, WAV, and common output targets such as original size, 2160p, 1080p, 720p, and 480p.
- **Multi-view resource management**: task and file views unify downloaded, transcoded, subtitle, cover, and imported files. Supports media preview, task details, file details, batch selection, deletion, failed-task recovery, file existence checks, and stale-record cleanup.
- **Local music playback**: automatically indexes audio files in the library, with local playback, playback queue, artwork display, synced lyrics, East Asian romanized/pinyin lyrics, an equalizer, and multiple spectrum visualization styles.
- **YouTube Music**: provides a desktop YouTube Music experience with account connections, song/artist/playlist search, home recommendations, playlist library, followed artists, liked music, playback queue, lyrics, and ad-data cleanup to reduce playback interruptions.
- **YouTube Live**: supports custom YouTube Live groups and channels, live status viewing, live radio playback, and direct live video viewing.
- **Automatic dependency management**: automatically maintains installation, verification, and upgrades for YT-DLP, FFmpeg, Bun, and related tools. Tool paths are managed independently by the app and do not depend on or pollute the user's global environment.
- **Credential and user isolation**: supports calling local browser capabilities through CDP while persisting independent Profiles and Cookies. Data only comes from user-initiated sign-in, and connection settings stay isolated from everyday browser use.
- **Appearance customization**: supports theme packs, light/dark/auto modes, accent colors, fonts, font sizes, sidebar styles, and more. The built-in Codex Pets Gallery can import online and local Pets as desktop companion elements.

## Product Preview

<p align="center">
  <img src="./images/download.webp" alt="XiaDown download task view" width="88%" />
  <br />
  <sub>Download and transcoding tasks</sub>
</p>

<p align="center">
  <img src="./images/sniff-desk.webp" alt="XiaDown sniffing desk resource capture view" width="88%" />
  <br />
  <sub>Sniffing desk for capturing page resources</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="XiaDown YouTube Music playback view" width="88%" />
  <br />
  <sub>Desktop YouTube Music playback</sub>
</p>

<p align="center">
  <img src="./images/youtube-live.webp" alt="XiaDown YouTube Live video view" width="88%" />
  <br />
  <sub>YouTube Live video viewing</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="XiaDown library view" width="88%" />
  <br />
  <sub>Unified library for downloaded and transcoded content</sub>
</p>

<details>
  <summary>More settings and personalization views</summary>

  <p align="center">
    <img src="./images/connector.webp" alt="XiaDown connection and account isolation view" width="88%" />
    <br />
    <sub>Connection settings, Cookies, and browser Profile isolation</sub>
  </p>

  <p align="center">
    <img src="./images/tools.webp" alt="XiaDown dependency tool management view" width="88%" />
    <br />
    <sub>Automatic management for YT-DLP, FFmpeg, and Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="XiaDown appearance settings view" width="88%" />
    <br />
    <sub>Themes, light/dark mode, accent colors, fonts, and font sizes</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="XiaDown Codex Pets Gallery view" width="88%" />
    <br />
    <sub>Codex Pets Gallery and local pet imports</sub>
  </p>
</details>

## Quick Start

### Download and install

Download the latest installer directly below. Older releases are available on [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Platform | Architecture | Package | Download |
| --- | --- | --- | --- |
| macOS | Apple Silicon | Archive | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | Archive | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | Installer | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portable | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### First launch

1. `macOS`: unzip the package and move `XiaDown.app` to the Applications folder. If macOS says the app cannot be opened or is damaged, run `sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app`.
2. `Windows`: run the `.exe` installer directly, or unzip the portable package and launch it. If SmartScreen appears on first launch, choose `More info -> Run anyway`.
3. XiaDown opens an onboarding flow for language, theme, proxy, and dependency setup. The main workflows are in the onboarding flow and UI.

## Disclaimer

- XiaDown is provided as a media management and download assistant, for learning, research, and saving content that you are authorized to access and use.
- You are responsible for ensuring any download, storage, conversion, or use of content is authorized by the rights holder and complies with applicable laws and the terms of the target site/platform.
- Do not use XiaDown to download, distribute, sell, or otherwise exploit infringing, unauthorized, paid/restricted, private, or otherwise unlawful content.
- Any copyright, platform policy, account, network, or other legal responsibility arising from your use of XiaDown is borne by you; the project maintainers are not responsible for user conduct or its consequences.

## Acknowledgements

XiaDown is built on top of excellent open-source projects. The desktop experience, media pipeline, local storage, browser connections, online music, and frontend interface all depend on these foundations.

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

- Website: <https://xiadown.dreamapp.cc/>
- Email: <xunruhao@gmail.com>
