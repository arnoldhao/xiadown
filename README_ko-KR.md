<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDown 아이콘" />
  <h1>XiaDown</h1>
  <p><strong>동영상 다운로드를 지원하는 미디어 라이브러리 관리 앱</strong></p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="최신 버전" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="라이선스" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="지원 플랫폼" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="기술 스택" />
  </p>
  <p>
    <a href="https://xiadown.app/">공식 웹사이트</a> ·
    <a href="https://xiadown.app/docs/">사용 설명서</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">릴리스</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">문제 제보</a> ·
    <a href="https://ko-fi.com/arnoldhao">후원</a>
  </p>
  <p>
    <a href="./README.md">简体中文</a> ·
    <a href="./README_zh-Hant.md">繁體中文</a> ·
    <a href="./README_en.md">English</a> ·
    <a href="./README_ja-JP.md">日本語</a> ·
    <strong>한국어</strong> ·
    <a href="./README_es-419.md">Español (LatAm)</a> ·
    <a href="./README_pt-BR.md">Português (BR)</a> ·
    <a href="./README_id-ID.md">Bahasa Indonesia</a> ·
    <a href="./README_vi-VN.md">Tiếng Việt</a>
  </p>
</div>

<p align="center">
  <a href="https://xiadown.app/docs/library/">
    <img src="./images/library.webp" alt="XiaDown 라이브러리 화면" width="92%" />
  </a>
  <br />
  <strong>라이브러리</strong>
</p>

## 프로젝트 소개

XiaDown은 로컬 우선 미디어 라이브러리 관리 앱으로, 동영상 다운로드와 미디어 스니핑을 지원합니다. YouTube, YouTube Music, RSS 클라이언트로도 사용할 수 있으며, 콘텐츠를 둘러보는 중 필요한 미디어를 클릭 한 번으로 다운로드할 수 있습니다.

## 핵심 기능

- 🗂️ **[라이브러리](https://xiadown.app/docs/library/)** — 동영상 다운로드, 트랜스코딩 및 라이브러리 관리.
- 🔎 **[스니핑](https://xiadown.app/docs/sniff/)** — 웹페이지 미디어 탐지 및 다운로드.
- 🎵 **[음악](https://xiadown.app/docs/music/)** — YouTube Music 탐색 및 다운로드, 로컬 음악 재생.
- ▶️ **[YouTube](https://xiadown.app/docs/youtube/)** — 동영상 탐색, 재생 및 다운로드.
- 📡 **[RSS](https://xiadown.app/docs/rss/)** — 콘텐츠 구독, 열람 및 미디어 다운로드.

## 모바일

📱 iPhone 및 iPad 클라이언트를 개발하고 있습니다.

## 제품 화면

<table>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/sniff/">
        <img src="./images/sniff-desk.webp" alt="XiaDown 스니핑 화면" width="100%" />
      </a>
      <br />
      <strong>스니핑</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/rss/">
        <img src="./images/rss.webp" alt="XiaDown RSS 구독 화면" width="100%" />
      </a>
      <br />
      <strong>RSS</strong>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/music/">
        <img src="./images/youtube-music.webp" alt="XiaDown 음악 화면" width="100%" />
      </a>
      <br />
      <strong>음악</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/youtube/">
        <img src="./images/youtube-live.webp" alt="XiaDown YouTube 동영상 탐색 화면" width="100%" />
      </a>
      <br />
      <strong>YouTube</strong>
    </td>
  </tr>
</table>

## 설치

### Homebrew

macOS에서는 Homebrew cask로 설치할 수 있습니다:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### 설치 패키지 다운로드

| 플랫폼 | 아키텍처 | 형식 | 다운로드 |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | 설치 프로그램 | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | 포터블 버전 | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

> macOS 버전은 macOS 14(Sonoma) 이상, Windows 버전은 Windows 10 이상이 필요합니다.

처음 실행하면 언어, 외관, 네트워크 및 실행에 필요한 도구를 구성하는 안내가 시작됩니다. 자세한 내용은 [설치 및 첫 실행](https://xiadown.app/docs/start/install/)을 참고하세요.

## 로컬 개발

개발 환경에는 Go 1.25.12, Node.js 24, Bun 1.3.5 및 Wails 3 alpha2.117이 필요합니다:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
wails3 task dev
```

다른 빌드 및 검사 작업은 [Taskfile.yml](./Taskfile.yml)을 참고하세요.

## 면책 조항

- XiaDown은 미디어를 관리하고 사용자가 접근 및 사용할 권한이 있는 콘텐츠를 저장하는 용도로만 사용해야 합니다.
- 콘텐츠의 다운로드, 저장, 변환 및 사용이 거주 지역의 법률, 권리자의 허가, 대상 플랫폼의 서비스 약관을 준수하는지 사용자가 직접 확인해야 합니다.
- 침해 콘텐츠, 무단 콘텐츠, 유료 또는 접근 제한 콘텐츠, 개인정보 관련 콘텐츠 및 기타 불법 콘텐츠를 처리하는 데 XiaDown을 사용하지 마세요.
- XiaDown 사용으로 발생하는 저작권, 플랫폼 정책, 계정, 네트워크 및 기타 책임은 사용자에게 있습니다.

## 감사의 글

XiaDown은 [Go](https://go.dev/), [Wails](https://v3alpha.wails.io/), [React](https://react.dev/), [yt-dlp](https://github.com/yt-dlp/yt-dlp), [FFmpeg](https://ffmpeg.org/), [SQLite](https://www.sqlite.org/) 등의 오픈 소스 프로젝트를 사용합니다. 의존성과 라이선스 정보는 [THIRD_PARTY_NOTICES.txt](./frontend/public/THIRD_PARTY_NOTICES.txt)를 참고하세요.

## 협업

- 현재 이 프로젝트는 PR을 받고 있지 않습니다. 문제와 제안은 [GitHub Issues](https://github.com/arnoldhao/xiadown/issues)에 남겨 주세요.
- 이 저장소는 `Apache-2.0` 라이선스를 따릅니다. 자세한 내용은 [LICENSE](./LICENSE)을 참고하세요.

## 연락처

- 공식 웹사이트: <https://xiadown.app/>
- 사용 설명서: <https://xiadown.app/docs/>
- 이메일: <xunruhao@gmail.com>
