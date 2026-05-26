<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDownアイコン" />
  <h1>XiaDown</h1>
  <p><strong>オンライン音楽に対応したデュアルエンジン動画ダウンロードツールです。</strong></p>
  <p>Listen Keep, Make it Yours</p>
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
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="最新バージョン" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="ライセンス" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="対応プラットフォーム" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="技術スタック" />
  </p>
</div>

## 概要

XiaDownはオンライン音楽プレーヤーであり、デュアルエンジンの動画ダウンロードツールでもあります。

コンテンツクリエイターのために作られています。素材が必要なときは、ブラウザスニッフィングとYT-DLPによる強力なダウンロード機能を提供し、作業中はバックグラウンドでオンライン音楽を再生できます。ライブラリ、トランスコード、依存ツールの自動管理、アカウント分離、ペット、外観カスタマイズにより、素材処理にも、普段使いのデスクトップメディアツールとしても使えます。

## 主な機能

- 📥 **デュアルエンジン動画ダウンロード**: YT-DLPダウンロードエンジンに加え、CDPベースのブラウザスニッフィングエンジンを内蔵しています。通常のリンクは直接解析してダウンロードでき、接続設定に保存されたCookiesも利用できます。動的に読み込まれるページ、複雑なサイト構造、実ブラウザセッションを必要とするリソースでは、スニッフィングモードで動画、音声、字幕、カバー、その他のメディアリソースを捕捉し、ダウンロード後にトランスコードとライブラリ管理へつなげられます。
- 🎧 **デスクトップ音楽プレーヤー**: ダウンロードリソース内のローカル音声を自動管理し、YouTube Musicでの曲、アーティスト、プレイリスト検索、YouTube Liveのライブラジオ再生とライブ動画表示に対応します。プレーヤーはキュー、カバー、同期歌詞、東アジア言語のローマ字/ピンイン歌詞、イコライザー、スペクトラム可視化を備えています。
- 🧩 **自由で管理しやすい利用空間**: 依存ツールはアプリが自動でインストール・アップグレードし、システム環境を汚しません。アカウント、Cookies、ブラウザProfileは独立した接続設定で管理され、テーマ、ライト/ダークモード、アクセントカラー、フォント、文字サイズ、Codex Petsも自由に調整できます。

## 中核機能

- **スニッフィングダウンロード**: CDPベースで自主開発したブラウザスニッフィング機能により、ページ内の動画、音声、ライブストリーム、マニフェスト、画像、字幕、APIレスポンスなどを観測できます。ユーザーが明示的にログインした実ブラウザ環境で、TikTok、抖音、快手、小紅書などのサイトリソースを識別・ダウンロードでき、ダウンロード完了後に自動でトランスコードへ進めることもできます。
- **YT-DLPダウンロード**: YT-DLPを統合し、多数のオンライン動画サイトから素材をダウンロードできます。YouTube、Bilibiliなどのよく使われるプラットフォームも安定してダウンロードできます。リンクを貼り付けるだけで動画、音声、字幕、カバーを解析・保存でき、接続設定に保存されたCookiesを使ってユーザーがアクセス権を持つコンテンツを取得し、その後トランスコードとライブラリ管理へ進められます。
- **音声・動画トランスコード**: FFmpegを基盤に、ダウンロード後の連動トランスコードとローカルファイルの手動選択に対応します。H.264、H.265、VP9、MP3、AAC、Opus、FLAC、WAVなどのプリセットを内蔵し、オリジナルサイズ、2160p、1080p、720p、480pなどの出力に対応します。
- **複数ビューのリソース管理**: タスクビューとファイルビューで、ダウンロード、トランスコード、字幕、カバー、インポートファイルを一元管理できます。メディアプレビュー、タスク詳細、ファイル詳細、一括選択、削除、失敗タスクの復旧、ファイル存在チェック、無効レコードの整理に対応します。
- **ローカル音楽再生**: ライブラリ内の音声ファイルを自動でインデックス化し、ローカル再生、再生キュー、カバー表示、同期歌詞、東アジア言語のローマ字/ピンイン歌詞、イコライザー、複数のスペクトラム可視化を利用できます。
- **YouTube Music**: デスクトップ向けのYouTube Music体験を提供します。アカウント接続、曲/アーティスト/プレイリスト検索、ホームおすすめ、プレイリストライブラリ、フォロー中のアーティスト、好きな音楽、再生キュー、歌詞に対応し、広告データのクリーンアップで再生の妨げを減らします。
- **YouTube Live**: YouTube Liveのグループとチャンネルを自由に追加でき、ライブ状態の確認、ライブラジオ再生、ライブ動画の直接表示に対応します。
- **依存ツールの自動管理**: YT-DLP、FFmpeg、Bunなどのツールのインストール、検証、アップグレードを自動で管理します。ツールパスはアプリが独立して管理するため、ユーザーのグローバル環境に依存せず、汚しません。
- **認証情報とユーザー環境の分離**: CDPを通じてローカルブラウザの機能を呼び出し、独立したProfilesとCookiesを永続化できます。データはユーザーが明示的にログインしたものに限られ、接続設定は普段使いのブラウザ環境から分離されます。
- **外観の自由なカスタマイズ**: テーマパック、ライト/ダーク/自動モード、アクセントカラー、フォント、文字サイズ、サイドバースタイルなどを設定できます。内蔵のCodex Pets GalleryからオンラインやローカルのPetsを取り込み、デスクトップ上の相棒として使えます。

## 製品プレビュー

<p align="center">
  <img src="./images/download.webp" alt="XiaDownのダウンロードタスク画面" width="88%" />
  <br />
  <sub>ダウンロードとトランスコードのタスク</sub>
</p>

<p align="center">
  <img src="./images/sniff-desk.webp" alt="XiaDownのスニッフィングデスクによるリソース捕捉画面" width="88%" />
  <br />
  <sub>スニッフィングデスクでページリソースを捕捉</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="XiaDownのYouTube Music再生画面" width="88%" />
  <br />
  <sub>YouTube Musicのデスクトップ再生</sub>
</p>

<p align="center">
  <img src="./images/youtube-live.webp" alt="XiaDownのYouTube Live動画表示画面" width="88%" />
  <br />
  <sub>YouTube Liveのライブ動画表示</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="XiaDownのライブラリ画面" width="88%" />
  <br />
  <sub>ダウンロード済み・トランスコード済みコンテンツをライブラリで一元管理</sub>
</p>

<details>
  <summary>その他の設定とパーソナライズ画面</summary>

  <p align="center">
    <img src="./images/connector.webp" alt="XiaDownの接続とアカウント分離画面" width="88%" />
    <br />
    <sub>接続設定、Cookies、ブラウザProfileの分離</sub>
  </p>

  <p align="center">
    <img src="./images/tools.webp" alt="XiaDownの依存ツール管理画面" width="88%" />
    <br />
    <sub>YT-DLP、FFmpeg、Bunの依存ツール自動管理</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="XiaDownの外観設定画面" width="88%" />
    <br />
    <sub>テーマ、ライト/ダークモード、アクセントカラー、フォント、文字サイズ</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="XiaDownのCodex Pets Gallery画面" width="88%" />
    <br />
    <sub>Codex Pets GalleryとローカルPetのインポート</sub>
  </p>
</details>

## クイックスタート

### ダウンロードとインストール

以下から最新パッケージを直接ダウンロードできます。過去のリリースは[GitHub Releases](https://github.com/arnoldhao/xiadown/releases)で確認できます。

| プラットフォーム | アーキテクチャ | パッケージ | ダウンロード |
| --- | --- | --- | --- |
| macOS | Apple Silicon | アーカイブ | [ダウンロード](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | アーカイブ | [ダウンロード](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | インストーラー | [ダウンロード](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | ポータブル | [ダウンロード](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### 初回起動

1. `macOS`: パッケージを展開し、`XiaDown.app`をアプリケーションフォルダへ移動します。macOSがアプリを開けない、または破損していると表示する場合は、ターミナルで`sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app`を実行してください。
2. `Windows`: `.exe`インストーラーを直接実行するか、ポータブルパッケージを展開して起動します。初回起動時にSmartScreenが表示された場合は、`詳細情報 -> 実行`を選択します。
3. XiaDownは言語、テーマ、プロキシ、依存関係を設定するオンボーディングを開きます。主要な操作はオンボーディングとアプリ内UIにまとまっています。

## 免責事項

- 下蛋はメディア管理およびダウンロードを補助するツールとして提供され、学習、研究、および利用者本人がアクセス・利用する権利を有するコンテンツの保存を目的としています。
- コンテンツのダウンロード、保存、変換、利用については、権利者の許諾を得ていること、および適用される法令と対象サイト/プラットフォームの利用規約に従っていることを、利用者自身の責任で確認してください。
- 侵害コンテンツ、無許諾コンテンツ、有料または制限付きコンテンツ、プライバシーに関わるコンテンツ、その他違法または不適切なコンテンツのダウンロード、配布、販売、利用に下蛋を使用しないでください。
- 下蛋の使用により生じる著作権、プラットフォーム規約、アカウント、ネットワーク、その他の法的責任は利用者が負うものとし、プロジェクトのメンテナーは利用者の行為およびその結果について責任を負いません。

## 謝辞

XiaDownは優れたオープンソースプロジェクトの上に構築されています。デスクトップ体験、メディア処理、ローカルストレージ、ブラウザ接続、オンライン音楽、フロントエンドUIは、これらの基盤に支えられています。

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

- Webサイト: <https://xiadown.dreamapp.cc/>
- メール: <xunruhao@gmail.com>
