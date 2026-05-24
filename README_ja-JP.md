<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDownアイコン" />
  <h1>XiaDown</h1>
  <p><strong>オンライン音楽に対応した動画ダウンロードツールです。</strong></p>
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

XiaDownはオンライン音楽プレーヤーであり、動画ダウンロードツールでもあります。

コンテンツクリエイターのために作られています。素材が必要なときは、YT-DLPを基盤にした強力なダウンロード機能を提供し、作業中はバックグラウンドでオンライン音楽を再生できます。ペットと外観カスタマイズにより、シンプルでありながら退屈しないアプリになっています。

## 主な機能

- **オンライン音楽プレーヤー**: YouTube Lo-FiステーションとYouTube Music向けのデスクトッププレーヤーです。アカウントログイン、曲、アーティスト、プレイリストの検索、再生キュー、歌詞、アートワーク表示に対応し、カスタムのオンラインLo-Fiステーションも追加できます。長く残したいトラックはローカルライブラリへダウンロードできます。
- **動画と音声のダウンロード**: YT-DLPを利用し、数千のオンライン動画サイトから素材をダウンロードできます。リンクを貼り付けるだけで動画、音声、字幕、カバーを保存し、その後トランスコードしてローカルライブラリで管理できます。
- **自分用のメディア空間**: 丁寧に設計されたテーマパック、アクセントカラー、外観モード、サイドバースタイル、Codex Petsの完全サポートを備えています。依存関係とアプリ更新は自動的に維持され、長く日常的に使えるメディアツールとして利用できます。

## 製品プレビュー

<p align="center">
  <img src="./images/download.png" alt="XiaDownのダウンロードタスク画面" width="88%" />
</p>

<p align="center">
  <img src="./images/listen.png" alt="XiaDown Listenのオンライン音楽再生画面" width="88%" />
</p>

<p align="center">
  <img src="./images/library.png" alt="XiaDownのライブラリ画面" width="88%" />
</p>

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
