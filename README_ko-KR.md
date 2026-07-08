<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDown 아이콘" />
  <h1>XiaDown</h1>
  <p><strong>온라인 음악을 지원하는 듀얼 엔진 동영상 다운로드 도구입니다.</strong></p>
  <p>Listen Keep, Make it Yours</p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="최신 버전" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="라이선스" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="지원 플랫폼" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="기술 스택" />
  </p>
  <p>
    <a href="https://xiadown.app/">웹사이트</a> ·
    <a href="https://xiadown.app/ko-kr/docs/">사용 가이드</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">릴리스</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">이슈</a> ·
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
  <img src="./images/download.webp" alt="XiaDown 다운로드 작업 화면" width="92%" />
  <br />
  <sub>다운로드 및 트랜스코딩 작업</sub>
</p>

## 개요

XiaDown은 온라인 음악 플레이어이자 듀얼 엔진 동영상 다운로드 도구입니다.

콘텐츠 크리에이터를 위한 일상 도구입니다. 자료가 필요할 때는 스니핑과 YT-DLP로 다운로드하고, 집중이 필요할 때는 온라인 음악을 백그라운드에서 재생할 수 있습니다. 풍부한 사용자 지정 옵션은 오래 사용해도 편하고 새롭게 느껴지도록 돕습니다.

## 주요 기능

### 📥 다운로드 및 트랜스코딩

- **스니핑 다운로드**: CDP로 페이지의 동영상, 오디오, 라이브 스트림, 매니페스트, 이미지, 자막, API 응답을 관찰합니다. 실제 브라우저 세션이 필요한 TikTok, Douyin, Kuaishou, Xiaohongshu 같은 사이트에 적합합니다.
- **YT-DLP 다운로드**: 링크를 붙여 넣으면 YouTube, Bilibili 같은 주요 플랫폼을 분석하고 동영상, 오디오, 자막, 커버를 저장합니다. 로그인된 신원을 사용해 접근 권한이 있는 콘텐츠를 다운로드할 수도 있습니다.
- **오디오/동영상 트랜스코딩**: FFmpeg 기반으로 다운로드 후 연동 트랜스코딩과 로컬 파일 트랜스코딩을 지원하며, H.264, H.265, VP9, MP3, AAC, Opus, FLAC, WAV 등의 프리셋을 제공합니다.

### 🗂️ 리소스 관리

- **다중 보기 리소스 관리**: 작업 보기와 파일 보기로 다운로드, 트랜스코딩, 자막, 커버, 가져온 파일을 통합 관리합니다. 미리보기, 상세 정보, 일괄 선택, 삭제, 실패 복구, 만료 기록 정리를 지원합니다.

### 🎧 플레이어

- **로컬 음악 재생**: 라이브러리 오디오를 자동으로 인덱싱하고, 대기열, 커버, 동기화 가사, 동아시아 로마자/병음 가사, 이퀄라이저, 스펙트럼 시각화를 지원합니다.
- **YouTube Music**: 데스크톱형 경험으로 곡, 아티스트, 플레이리스트를 검색하고, 홈 추천, 플레이리스트 라이브러리, 팔로우한 아티스트, 좋아요한 음악, 재생 대기열, 가사를 제공합니다.
- **YouTube Live**: 라이브 그룹과 채널을 직접 만들고, 라이브 상태 확인, 라이브 라디오 재생, 라이브 동영상 직접 보기를 지원합니다.

### 🔐 안전 및 격리

- **의존성 자동 관리**: YT-DLP, FFmpeg, Bun 등의 도구를 자동으로 설치, 검증, 업그레이드합니다. 도구 경로는 앱이 독립적으로 관리하므로 시스템 환경을 오염시키지 않습니다.
- **자격 증명과 사용자 환경 격리**: 앱 세션 데이터는 사용자가 직접 로그인한 것에서만 생성되며, macOS와 Windows에서는 시스템 암호화 기능으로 독립 저장됩니다. 연결 설정은 일상적인 브라우저 사용과 분리됩니다.

### 🎨 자유도

- **외관 자유 설정**: 테마 팩, 라이트/다크/자동 모드, 강조 색상, 글꼴, 글꼴 크기, 사이드바 스타일을 지원합니다. 내장된 Codex Pets Gallery에서 온라인 및 로컬 Pets를 가져올 수 있습니다.

## 제품 미리보기

<p align="center">
  <img src="./images/sniff-desk.webp" alt="XiaDown 스니핑 데스크 리소스 포착 화면" width="92%" />
  <br />
  <sub>스니핑 데스크로 웹페이지 리소스 포착</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="XiaDown YouTube Music 재생 화면" width="92%" />
  <br />
  <sub>YouTube Music 데스크톱 재생</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="XiaDown 라이브러리 화면" width="92%" />
  <br />
  <sub>다운로드 및 트랜스코딩 콘텐츠를 라이브러리에서 통합 관리</sub>
</p>

<details>
  <summary><strong>더 많은 화면</strong></summary>

  <p align="center">
    <img src="./images/youtube-live.webp" alt="XiaDown YouTube Live 동영상 보기 화면" width="92%" />
    <br />
    <sub>YouTube Live 라이브 동영상 보기</sub>
  </p>

  <p align="center">
    <img src="./images/app_sessions.webp" alt="XiaDown 앱 세션 및 로그인 상태 화면" width="92%" />
    <br />
    <sub>앱 세션, 로그인 확인 및 계정 상태 관리</sub>
  </p>

  <p align="center">
    <img src="./images/settings_download.webp" alt="XiaDown 다운로드 설정 및 의존성 도구 관리 화면" width="92%" />
    <br />
    <sub>다운로드 폴더, 동시 작업 수, YT-DLP, FFmpeg, Bun 관리</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="XiaDown 외관 설정 화면" width="92%" />
    <br />
    <sub>테마, 라이트/다크 모드, 강조 색상, 글꼴, 글꼴 크기 설정</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="XiaDown Codex Pets Gallery 화면" width="92%" />
    <br />
    <sub>Codex Pets Gallery 및 로컬 Pet 가져오기</sub>
  </p>
</details>

## 설치 방법

### Homebrew

macOS에서는 Homebrew cask로 설치할 수 있습니다:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### 설치 패키지 다운로드

아래에서 최신 설치 패키지를 바로 다운로드할 수 있습니다. 이전 버전은 [GitHub Releases](https://github.com/arnoldhao/xiadown/releases)에서 확인할 수 있습니다.

| 플랫폼 | 아키텍처 | 패키지 | 다운로드 |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | 설치 프로그램 | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | 포터블 | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### 처음 실행

1. `macOS`: `.dmg`를 열고 `XiaDown.app`을 응용 프로그램 폴더로 드래그한 뒤 실행합니다.
2. `Windows`: `.exe` 설치 프로그램을 직접 실행하거나 포터블 패키지를 압축 해제한 뒤 실행합니다. 처음 실행할 때 SmartScreen이 나타나면 `추가 정보 -> 실행`을 선택하세요.
3. XiaDown은 언어, 테마, 프록시, 의존성 설정을 위한 온보딩 흐름을 엽니다. 주요 작업 흐름은 온보딩과 앱 UI에 모여 있습니다.

### CDP 브라우저 지원

현재 지원되는 브라우저:

| 주요 | 개인정보 보호와 효율 | 특화 및 지역 |
| --- | --- | --- |
| Chrome, Chromium, Edge | Brave, Vivaldi, Arc, Helium | Opera, Opera GX, Yandex Browser |

## 로컬 개발

Go와 Bun 환경을 준비한 뒤 Wails 3 CLI를 설치하고 개발 모드를 시작합니다:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.95
wails3 dev
```

## 면책 조항

- XiaDown은 미디어 관리 및 다운로드 보조 도구로 제공되며, 학습, 연구, 그리고 사용자가 접근 및 사용할 권한이 있는 콘텐츠를 저장하기 위한 용도입니다.
- 콘텐츠의 다운로드, 저장, 변환, 사용이 권리자의 허가를 받았고 적용되는 법률 및 대상 사이트/플랫폼의 서비스 약관을 준수하는지 확인할 책임은 사용자에게 있습니다.
- 침해 콘텐츠, 무단 콘텐츠, 유료 또는 제한 콘텐츠, 개인정보와 관련된 콘텐츠, 기타 불법 콘텐츠를 다운로드, 배포, 판매하거나 이용하기 위해 XiaDown을 사용하지 마세요.
- XiaDown 사용으로 발생하는 저작권, 플랫폼 정책, 계정, 네트워크 또는 기타 법적 책임은 사용자 본인에게 있으며, 프로젝트 관리자는 사용자의 행위와 그 결과에 책임을 지지 않습니다.

## 감사의 글

XiaDown은 훌륭한 오픈 소스 프로젝트 위에 만들어졌습니다. 데스크톱 경험, 미디어 처리, 로컬 저장소, 브라우저 연결, 온라인 음악, 인터페이스 기능은 모두 이러한 기반에 의존합니다.

| 분류 | 홈페이지 |
| --- | --- |
| 데스크톱 프레임워크 | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| 미디어 처리 | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| 로컬 저장소 | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| 브라우저 연결 | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| 프론트엔드 경험 | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## 협업

- 이 프로젝트는 계속 개발 중이며 현재 pull request는 받지 않습니다. 피드백, 버그 제보, 사용 사례는 [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) 또는 이메일로 환영합니다.
- 이 저장소는 `Apache-2.0` 라이선스를 따릅니다. 자세한 내용은 [LICENSE](./LICENSE)를 참고하세요.

## 연락처

- 웹사이트: <https://xiadown.app/>
- 사용 가이드: <https://xiadown.app/ko-kr/docs/>
- 이메일: <xunruhao@gmail.com>
