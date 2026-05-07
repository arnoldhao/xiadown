<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="下蛋 / XiaDown 图标" />
  <h1>下蛋 / XiaDown</h1>
  <p><strong>一款支持在线音乐的视频下载工具</strong></p>
  <p>Listen Keep, Make it Yours · 随你，听存随心</p>
  <p>
    <strong>简体中文</strong> ·
    <a href="./README_en.md">English</a>
  </p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="最新版本" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="许可证" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="支持平台" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="技术栈" />
  </p>
</div>

## 项目简介

下蛋是一款在线音乐播放器，也是一款视频下载工具。

它是为内容创作者打造的：需要素材时，提供基于 YT-DLP 的强大下载能力；需要工作时，在后台提供在线音乐播放能力。同时依托宠物与自定义外观，让软件保持简约，也不显得乏味。

## 主要能力

- 🎧 **在线音乐播放**：集成 YouTube Lo-Fi 电台与 YouTube Music，支持登录账号、搜索歌曲/艺人/歌单、播放队列、歌词封面，并可把想留下的在线曲目继续下载到本地。
- 📥 **视频与音频下载**：基于 YT-DLP，支持上千个在线视频网站的素材下载；粘贴链接即可保存视频、音频、字幕和封面，下载后可继续转码并在资源库里统一管理。
- 🧩 **个性化使用空间**：提供主题包、强调色、外观模式、侧边栏样式、宠物和连接能力，依赖与更新会自动维护，适合长期作为自己的媒体工具使用。

## 产品界面

<p align="center">
  <img src="./images/download.png" alt="下蛋下载任务界面" width="88%" />
</p>

<p align="center">
  <img src="./images/dreamfm.png" alt="下蛋 Dream.FM 在线音乐播放界面" width="88%" />
</p>

<p align="center">
  <img src="./images/library.png" alt="下蛋资源库界面" width="88%" />
</p>

## 快速开始

### 下载安装

可直接下载最新安装包；历史版本见 [GitHub 发布页](https://github.com/arnoldhao/xiadown/releases)。

| 平台 | 架构 | 形式 | 下载 |
| --- | --- | --- | --- |
| macOS | Apple 芯片 | 压缩包 | [点击下载](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | 压缩包 | [点击下载](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | 安装版 | [点击下载](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | 便携版 | [点击下载](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### 首次打开

1. `macOS`：解压后将 `XiaDown.app` 移动到“应用程序”目录。若系统提示“无法打开”或“已损坏”，请在终端执行 `sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app`。
2. `Windows`：安装版直接运行 `.exe`；便携版解压后直接启动。若首次启动出现 SmartScreen，选择“更多信息 -> 仍要运行”。
3. 首次启动会进入欢迎引导，完成语言、主题、代理和依赖安装后即可进入主界面。主要流程都集中在欢迎引导和界面内。

## 感谢

下蛋建立在一系列优秀的开源项目之上。桌面体验、媒体处理、本地存储、浏览器连接、在线音乐与界面能力，都离不开这些依赖的支持。

| 分类 | 项目主页 |
| --- | --- |
| 桌面框架 | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| 媒体处理 | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| 本地存储 | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| 浏览器连接 | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| 前端体验 | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## 协作

- 项目正在持续演进，当前暂不接受 PR，欢迎通过 [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) 或邮件反馈问题、分享建议与使用场景。
- 仓库采用 `Apache-2.0` 许可证，详见 [LICENSE](./LICENSE)。

## 联系

- 官网：<https://xiadown.dreamapp.cc/>
- 邮箱：<xunruhao@gmail.com>
