<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDownアイコン" />
  <h1>XiaDown</h1>
  <p><strong>オンライン音楽に対応したデュアルエンジン動画ダウンロードツールです。</strong></p>
  <p>Listen Keep, Make it Yours</p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="最新バージョン" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="ライセンス" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="対応プラットフォーム" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="技術スタック" />
  </p>
  <p>
    <a href="https://xiadown.app/">Webサイト</a> ·
    <a href="https://xiadown.app/ja-jp/docs/">使い方ガイド</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">リリース</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Issue</a> ·
    <a href="https://ko-fi.com/arnoldhao">スポンサー</a>
  </p>
  <p>
    <a href="./README.md">简体中文</a> ·
    <a href="./README_zh-Hant.md">繁體中文</a> ·
    <a href="./README_en.md">English</a> ·
    <strong>日本語</strong> ·
    <a href="./README_ko-KR.md">한국어</a> ·
    <a href="./README_es-419.md">Español (LatAm)</a> ·
    <a href="./README_pt-BR.md">Português (BR)</a> ·
    <a href="./README_id-ID.md">Bahasa Indonesia</a> ·
    <a href="./README_vi-VN.md">Tiếng Việt</a>
  </p>
</div>

<p align="center">
  <img src="./images/download.webp" alt="XiaDownのダウンロードタスク画面" width="92%" />
  <br />
  <sub>ダウンロードとトランスコードのタスク</sub>
</p>

## 概要

XiaDownはオンライン音楽プレーヤーであり、デュアルエンジンの動画ダウンロードツールでもあります。

コンテンツクリエイターの日常ツールとして作られています。素材が必要なときはスニッフィングとYT-DLPでダウンロードし、集中したいときはオンライン音楽をバックグラウンドで再生できます。豊富なカスタマイズにより、長く使っても軽やかで新鮮な使い心地を保てます。

## 主な機能

### 📥 ダウンロードとトランスコード

- **スニッフィングダウンロード**: CDPを使ってページ内の動画、音声、ライブストリーム、マニフェスト、画像、字幕、APIレスポンスを観測します。TikTok、抖音、快手、小紅書など、実ブラウザセッションが必要なサイトに向いています。
- **YT-DLPダウンロード**: リンクを貼り付けるだけでYouTubeやBilibiliなどの主要プラットフォームを解析し、動画、音声、字幕、カバーを保存できます。ログイン済みの本人情報を使って、アクセス権のあるコンテンツをダウンロードすることもできます。
- **音声・動画トランスコード**: FFmpegを基盤に、ダウンロード後の連動トランスコードとローカルファイルのトランスコードに対応します。H.264、H.265、VP9、MP3、AAC、Opus、FLAC、WAVなどのプリセットを内蔵しています。

### 🗂️ リソース管理

- **複数ビューのリソース管理**: タスクビューとファイルビューで、ダウンロード、トランスコード、字幕、カバー、インポートファイルを一元管理します。プレビュー、詳細表示、一括選択、削除、失敗タスクの復旧、無効レコードの整理に対応します。

### 🎧 プレーヤー

- **ローカル音楽再生**: ライブラリ内の音声を自動でインデックス化し、キュー、カバー、同期歌詞、東アジア言語のローマ字/ピンイン歌詞、イコライザー、スペクトラム可視化に対応します。
- **YouTube Music**: デスクトップ向けの体験で曲、アーティスト、プレイリストを検索でき、ホームおすすめ、プレイリストライブラリ、フォロー中のアーティスト、好きな音楽、再生キュー、歌詞を利用できます。
- **YouTube Live**: ライブのグループとチャンネルを自由に作成し、ライブ状態の確認、ライブラジオ再生、ライブ動画の直接表示ができます。

### 🔐 安全性と分離

- **依存ツールの自動管理**: YT-DLP、FFmpeg、Bunなどのツールを自動でインストール、検証、アップグレードします。ツールパスはアプリが独立して管理し、システム環境を汚しません。
- **認証情報とユーザー環境の分離**: アプリセッションのデータはユーザーが明示的にサインインしたものに限られ、macOSとWindowsではシステムの暗号化機能で独立して保存されます。接続設定は普段のブラウザ利用から分離されます。

### 🎨 自由度

- **外観の自由なカスタマイズ**: テーマパック、ライト/ダーク/自動モード、アクセントカラー、フォント、文字サイズ、サイドバースタイルを設定できます。内蔵のCodex Pets GalleryからオンラインやローカルのPetsを取り込めます。

## 製品プレビュー

<p align="center">
  <img src="./images/sniff-desk.webp" alt="XiaDownのスニッフィングデスクによるリソース捕捉画面" width="92%" />
  <br />
  <sub>スニッフィングデスクでページリソースを捕捉</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="XiaDownのYouTube Music再生画面" width="92%" />
  <br />
  <sub>YouTube Musicのデスクトップ再生</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="XiaDownのライブラリ画面" width="92%" />
  <br />
  <sub>ダウンロード済み・トランスコード済みコンテンツをライブラリで一元管理</sub>
</p>

<details>
  <summary><strong>その他の画面</strong></summary>

  <p align="center">
    <img src="./images/youtube-live.webp" alt="XiaDownのYouTube Live動画表示画面" width="92%" />
    <br />
    <sub>YouTube Liveのライブ動画表示</sub>
  </p>

  <p align="center">
    <img src="./images/app_sessions.webp" alt="XiaDownのアプリセッションとサインイン状態画面" width="92%" />
    <br />
    <sub>アプリセッション、サインイン検証、アカウント状態管理</sub>
  </p>

  <p align="center">
    <img src="./images/settings_download.webp" alt="XiaDownのダウンロード設定と依存ツール管理画面" width="92%" />
    <br />
    <sub>ダウンロード先、並列数、YT-DLP、FFmpeg、Bunの管理</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="XiaDownの外観設定画面" width="92%" />
    <br />
    <sub>テーマ、ライト/ダークモード、アクセントカラー、フォント、文字サイズ</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="XiaDownのCodex Pets Gallery画面" width="92%" />
    <br />
    <sub>Codex Pets GalleryとローカルPetのインポート</sub>
  </p>
</details>

## インストール方法

### Homebrew

macOSではHomebrew caskでインストールできます：

```bash
brew install --cask arnoldhao/tap/xiadown
```

### インストーラーのダウンロード

以下から最新パッケージを直接ダウンロードできます。過去のリリースは[GitHub Releases](https://github.com/arnoldhao/xiadown/releases)で確認できます。

| プラットフォーム | アーキテクチャ | パッケージ | ダウンロード |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [ダウンロード](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [ダウンロード](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | インストーラー | [ダウンロード](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | ポータブル | [ダウンロード](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### 初回起動

1. `macOS`: `.dmg`を開き、`XiaDown.app`をアプリケーションフォルダへドラッグして開きます。
2. `Windows`: `.exe`インストーラーを直接実行するか、ポータブルパッケージを展開して起動します。初回起動時にSmartScreenが表示された場合は、`詳細情報 -> 実行`を選択します。
3. XiaDownは言語、テーマ、プロキシ、依存関係を設定するオンボーディングを開きます。主要な操作はオンボーディングとアプリ内UIにまとまっています。

### CDP ブラウザー対応

現在対応しているブラウザー：

| 主流 | プライバシーと効率 | 特色・地域 |
| --- | --- | --- |
| Chrome、Chromium、Edge | Brave、Vivaldi、Arc、Helium | Opera、Opera GX、Yandex Browser |

## ローカル開発

GoとBunの環境を準備したら、Wails 3 CLIをインストールして開発モードを起動します：

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.95
wails3 dev
```

## 免責事項

- 下蛋はメディア管理およびダウンロードを補助するツールとして提供され、学習、研究、および利用者本人がアクセス・利用する権利を有するコンテンツの保存を目的としています。
- コンテンツのダウンロード、保存、変換、利用については、権利者の許諾を得ていること、および適用される法令と対象サイト/プラットフォームの利用規約に従っていることを、利用者自身の責任で確認してください。
- 侵害コンテンツ、無許諾コンテンツ、有料または制限付きコンテンツ、プライバシーに関わるコンテンツ、その他違法または不適切なコンテンツのダウンロード、配布、販売、利用に下蛋を使用しないでください。
- 下蛋の使用により生じる著作権、プラットフォーム規約、アカウント、ネットワーク、その他の法的責任は利用者が負うものとし、プロジェクトのメンテナーは利用者の行為およびその結果について責任を負いません。

## 謝辞

XiaDownは優れたオープンソースプロジェクトの上に構築されています。デスクトップ体験、メディア処理、ローカルストレージ、ブラウザ接続、オンライン音楽、インターフェイス機能は、これらの基盤に支えられています。

| カテゴリ | ホームページ |
| --- | --- |
| デスクトップフレームワーク | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| メディア処理 | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| ローカルストレージ | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| ブラウザ接続 | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| フロントエンド体験 | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## コラボレーション

- このプロジェクトは継続的に開発中で、現在はpull requestを受け付けていません。フィードバック、不具合報告、利用シナリオは[GitHub Issues](https://github.com/arnoldhao/xiadown/issues)またはメールで歓迎します。
- このリポジトリは`Apache-2.0`ライセンスです。詳しくは[LICENSE](./LICENSE)をご覧ください。

## 連絡先

- Webサイト: <https://xiadown.app/>
- 使い方ガイド: <https://xiadown.app/ja-jp/docs/>
- メール: <xunruhao@gmail.com>
