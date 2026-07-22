<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="下蛋 / XiaDown 图标" />
  <h1>下蛋 / XiaDown</h1>
  <p><strong>一款支持视频下载的资源库管理应用</strong></p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="最新版本" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="许可证" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="支持平台" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="技术栈" />
  </p>
  <p>
    <a href="https://xiadown.app/">官网</a> ·
    <a href="https://xiadown.app/docs/">使用文档</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">发布页</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">反馈问题</a> ·
    <a href="https://ko-fi.com/arnoldhao">赞助</a>
  </p>
  <p>
    <strong>简体中文</strong> ·
    <a href="./README_zh-Hant.md">繁體中文</a> ·
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
    <img src="./images/library.webp" alt="下蛋资源库界面" width="92%" />
  </a>
  <br />
  <strong>资源库</strong>
</p>

## 项目简介

下蛋是一款本地优先的资源库管理应用，支持视频下载与嗅探下载。下蛋同时还是 YouTube、YouTube Music 与 RSS 客户端，浏览内容时可一键下载所需媒体。

## 核心能力

- 🗂️ **[资源库](https://xiadown.app/docs/library/)** — 视频下载、转码与资源库管理。
- 🔎 **[嗅探](https://xiadown.app/docs/sniff/)** — 网页媒体嗅探与下载。
- 🎵 **[音乐](https://xiadown.app/docs/music/)** — YouTube Music 浏览与下载，本地音乐播放。
- ▶️ **[YouTube](https://xiadown.app/docs/youtube/)** — 视频浏览、播放与下载。
- 📡 **[RSS](https://xiadown.app/docs/rss/)** — 内容订阅、阅读与媒体下载。

## 移动端

📱 iPhone 与 iPad 客户端正在开发。

## 产品界面

<table>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/sniff/">
        <img src="./images/sniff-desk.webp" alt="下蛋嗅探界面" width="100%" />
      </a>
      <br />
      <strong>嗅探</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/rss/">
        <img src="./images/rss.webp" alt="下蛋 RSS 内容订阅界面" width="100%" />
      </a>
      <br />
      <strong>RSS</strong>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/music/">
        <img src="./images/youtube-music.webp" alt="下蛋音乐界面" width="100%" />
      </a>
      <br />
      <strong>音乐</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/youtube/">
        <img src="./images/youtube-live.webp" alt="下蛋 YouTube 视频浏览界面" width="100%" />
      </a>
      <br />
      <strong>YouTube</strong>
    </td>
  </tr>
</table>

## 安装

### Homebrew

macOS 可通过 Homebrew cask 安装：

```bash
brew install --cask arnoldhao/tap/xiadown
```

### 下载安装包

| 平台 | 架构 | 形式 | 下载 |
| --- | --- | --- | --- |
| macOS | Apple 芯片 | DMG | [点击下载](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [点击下载](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | 安装版 | [点击下载](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | 便携版 | [点击下载](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

> macOS 版本需要 macOS 14（Sonoma）或更高版本；Windows 版本需要 Windows 10 或更高版本。

首次启动会引导完成语言、外观、网络与运行依赖设置。详细步骤见[安装与首次启动](https://xiadown.app/docs/start/install/)。

## 本地开发

开发环境需要 Go 1.25.12、Node.js 24、Bun 1.3.5 与 Wails 3 alpha2.117：

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
wails3 task dev
```

其他构建与检查任务见 [Taskfile.yml](./Taskfile.yml)。

## 免责声明

- 下蛋仅用于管理媒体及保存本人有权访问和使用的内容。
- 使用者应自行确认下载、保存、转换和使用相关内容符合所在地法律法规、权利人授权及目标平台服务条款。
- 请勿使用下蛋处理侵权、未授权、付费受限、涉及隐私或其他违法违规内容。
- 因使用下蛋产生的版权、平台规则、账号、网络及其他责任由使用者自行承担。

## 感谢

下蛋使用了 [Go](https://go.dev/)、[Wails](https://v3alpha.wails.io/)、[React](https://react.dev/)、[yt-dlp](https://github.com/yt-dlp/yt-dlp)、[FFmpeg](https://ffmpeg.org/) 与 [SQLite](https://www.sqlite.org/) 等开源项目。依赖与许可证信息见 [THIRD_PARTY_NOTICES.txt](./frontend/public/THIRD_PARTY_NOTICES.txt)。

## 协作

- 项目当前暂不接受 PR，欢迎通过 [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) 反馈问题与建议。
- 仓库采用 `Apache-2.0` 许可证，详见 [LICENSE](./LICENSE)。

## 联系

- 官网：<https://xiadown.app/>
- 使用文档：<https://xiadown.app/docs/>
- 邮箱：<xunruhao@gmail.com>
