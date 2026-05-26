<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="XiaDown 아이콘" />
  <h1>XiaDown</h1>
  <p><strong>온라인 음악을 지원하는 듀얼 엔진 동영상 다운로드 도구입니다.</strong></p>
  <p>Listen Keep, Make it Yours</p>
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
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="최신 버전" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="라이선스" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="지원 플랫폼" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="기술 스택" />
  </p>
</div>

## 개요

XiaDown은 온라인 음악 플레이어이자 듀얼 엔진 동영상 다운로드 도구입니다.

콘텐츠 크리에이터를 위해 만들어졌습니다. 자료가 필요할 때는 브라우저 스니핑과 YT-DLP를 통한 강력한 다운로드 기능을 제공하고, 작업할 때는 온라인 음악을 백그라운드에서 재생합니다. 라이브러리, 트랜스코딩, 의존성 자동 관리, 계정 격리, 펫, 외관 사용자 지정을 바탕으로 XiaDown은 자료를 처리하는 도구이자 오래 쓰는 데스크톱 미디어 도구가 될 수 있습니다.

## 주요 기능

- 📥 **듀얼 엔진 동영상 다운로드**: YT-DLP 다운로드 엔진과 CDP 기반 브라우저 스니핑 엔진을 함께 제공합니다. 일반 링크는 직접 분석해 다운로드할 수 있고, 연결 설정에 저장된 Cookies도 사용할 수 있습니다. 동적 로딩, 복잡한 사이트 구조, 실제 브라우저 세션이 필요한 리소스는 스니핑 모드로 동영상, 오디오, 자막, 커버와 기타 미디어 리소스를 포착하고, 다운로드 후 트랜스코딩과 라이브러리 관리로 이어갈 수 있습니다.
- 🎧 **데스크톱 음악 플레이어**: 다운로드 리소스 안의 로컬 오디오를 자동으로 관리하고, YouTube Music의 곡, 아티스트, 플레이리스트 검색과 YouTube Live 라이브 라디오 재생 및 라이브 동영상 보기를 지원합니다. 플레이어는 대기열, 커버, 동기화 가사, 동아시아 로마자/병음 가사, 이퀄라이저, 스펙트럼 시각화를 제공합니다.
- 🧩 **자유롭고 제어 가능한 사용 공간**: 의존성 도구는 앱이 자동으로 설치하고 업그레이드하며 시스템 환경을 오염시키지 않습니다. 계정, Cookies, 브라우저 Profile은 독립된 연결 설정으로 관리되며, 테마, 라이트/다크 모드, 강조 색상, 글꼴, 글꼴 크기, Codex Pets도 자유롭게 조정할 수 있습니다.

## 핵심 기능

- **스니핑 다운로드**: 자체 개발한 CDP 기반 브라우저 스니핑 기능으로 페이지 안의 동영상, 오디오, 라이브 스트림, 매니페스트, 이미지, 자막, API 응답 등 다양한 리소스를 관찰할 수 있습니다. 사용자가 직접 로그인한 실제 브라우저 환경에서 TikTok, Douyin, Kuaishou, Xiaohongshu 등의 사이트 리소스를 식별하고 다운로드할 수 있으며, 다운로드가 끝난 뒤 자동으로 트랜스코딩 흐름으로 이어갈 수도 있습니다.
- **YT-DLP 다운로드**: YT-DLP를 통합해 다양한 온라인 동영상 사이트의 자료를 다운로드할 수 있고, YouTube와 Bilibili 같은 자주 쓰는 플랫폼도 안정적으로 지원합니다. 링크를 붙여 넣으면 동영상, 오디오, 자막, 커버를 분석해 저장하고, 연결 설정에 저장된 Cookies를 사용해 사용자가 접근 권한을 가진 콘텐츠를 다운로드한 뒤 트랜스코딩과 라이브러리 관리로 이어갈 수 있습니다.
- **오디오/동영상 트랜스코딩**: FFmpeg 기반으로 다운로드 후 연동 트랜스코딩과 로컬 파일 수동 선택 트랜스코딩을 지원합니다. H.264, H.265, VP9, MP3, AAC, Opus, FLAC, WAV 등 자주 쓰는 프리셋과 원본 크기, 2160p, 1080p, 720p, 480p 등 출력 시나리오를 제공합니다.
- **다중 보기 리소스 관리**: 작업 보기와 파일 보기를 통해 다운로드, 트랜스코딩, 자막, 커버, 가져온 파일을 통합 관리합니다. 미디어 미리보기, 작업 상세, 파일 상세, 일괄 선택, 삭제, 실패 작업 복구, 파일 존재 여부 검사, 만료된 기록 정리를 지원합니다.
- **로컬 음악 재생**: 라이브러리의 오디오 파일을 자동으로 인덱싱하고, 로컬 재생, 재생 대기열, 커버 표시, 동기화 가사, 동아시아 로마자/병음 가사, 이퀄라이저, 여러 스펙트럼 시각화 효과를 제공합니다.
- **YouTube Music**: 데스크톱형 YouTube Music 경험을 제공합니다. 계정 연결, 곡/아티스트/플레이리스트 검색, 홈 추천, 플레이리스트 라이브러리, 팔로우한 아티스트, 좋아요한 음악, 재생 대기열, 가사를 지원하며, 광고 데이터 정리로 재생 방해를 줄입니다.
- **YouTube Live**: YouTube Live 그룹과 채널을 자유롭게 추가하고, 라이브 상태 확인, 라이브 라디오 재생, 라이브 동영상 직접 보기를 지원합니다.
- **의존성 자동 관리**: YT-DLP, FFmpeg, Bun 등의 도구 설치, 검증, 업그레이드를 자동으로 관리합니다. 도구 경로는 앱이 독립적으로 관리하므로 사용자의 전역 환경에 의존하지도, 영향을 주지도 않습니다.
- **자격 증명과 사용자 환경 격리**: CDP를 통해 로컬 브라우저 기능을 호출하고 독립된 Profiles와 Cookies를 지속 저장할 수 있습니다. 데이터는 사용자가 직접 로그인한 것에서만 오며, 연결 설정은 일상적인 브라우저 사용 환경과 분리됩니다.
- **외관 자유 설정**: 테마 팩, 라이트/다크/자동 모드, 강조 색상, 글꼴, 글꼴 크기, 사이드바 스타일 등 외관 설정을 지원합니다. 내장된 Codex Pets Gallery에서 온라인 및 로컬 Pets를 가져와 데스크톱 동반 요소로 설정할 수 있습니다.

## 제품 미리보기

<p align="center">
  <img src="./images/download.webp" alt="XiaDown 다운로드 작업 화면" width="88%" />
  <br />
  <sub>다운로드 및 트랜스코딩 작업</sub>
</p>

<p align="center">
  <img src="./images/sniff-desk.webp" alt="XiaDown 스니핑 데스크 리소스 포착 화면" width="88%" />
  <br />
  <sub>스니핑 데스크로 웹페이지 리소스 포착</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="XiaDown YouTube Music 재생 화면" width="88%" />
  <br />
  <sub>YouTube Music 데스크톱 재생</sub>
</p>

<p align="center">
  <img src="./images/youtube-live.webp" alt="XiaDown YouTube Live 동영상 보기 화면" width="88%" />
  <br />
  <sub>YouTube Live 라이브 동영상 보기</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="XiaDown 라이브러리 화면" width="88%" />
  <br />
  <sub>다운로드 및 트랜스코딩 콘텐츠를 라이브러리에서 통합 관리</sub>
</p>

<details>
  <summary>더 많은 설정 및 개인화 화면</summary>

  <p align="center">
    <img src="./images/connector.webp" alt="XiaDown 연결 및 계정 격리 화면" width="88%" />
    <br />
    <sub>연결 설정, Cookies, 브라우저 Profile 격리</sub>
  </p>

  <p align="center">
    <img src="./images/tools.webp" alt="XiaDown 의존성 도구 관리 화면" width="88%" />
    <br />
    <sub>YT-DLP, FFmpeg, Bun 의존성 자동 관리</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="XiaDown 외관 설정 화면" width="88%" />
    <br />
    <sub>테마, 라이트/다크 모드, 강조 색상, 글꼴, 글꼴 크기 설정</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="XiaDown Codex Pets Gallery 화면" width="88%" />
    <br />
    <sub>Codex Pets Gallery 및 로컬 Pet 가져오기</sub>
  </p>
</details>

## 빠른 시작

### 다운로드 및 설치

아래에서 최신 설치 패키지를 바로 다운로드할 수 있습니다. 이전 버전은 [GitHub Releases](https://github.com/arnoldhao/xiadown/releases)에서 확인할 수 있습니다.

| 플랫폼 | 아키텍처 | 패키지 | 다운로드 |
| --- | --- | --- | --- |
| macOS | Apple Silicon | 압축 파일 | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | 압축 파일 | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | 설치 프로그램 | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | 포터블 | [다운로드](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### 처음 실행

1. `macOS`: 패키지를 압축 해제한 뒤 `XiaDown.app`을 응용 프로그램 폴더로 이동합니다. macOS에서 앱을 열 수 없거나 손상되었다고 표시되면 터미널에서 `sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app`을 실행하세요.
2. `Windows`: `.exe` 설치 프로그램을 직접 실행하거나 포터블 패키지를 압축 해제한 뒤 실행합니다. 처음 실행할 때 SmartScreen이 나타나면 `추가 정보 -> 실행`을 선택하세요.
3. XiaDown은 언어, 테마, 프록시, 의존성 설정을 위한 온보딩 흐름을 엽니다. 주요 작업 흐름은 온보딩과 앱 UI에 모여 있습니다.

## 면책 조항

- XiaDown은 미디어 관리 및 다운로드 보조 도구로 제공되며, 학습, 연구, 그리고 사용자가 접근 및 사용할 권한이 있는 콘텐츠를 저장하기 위한 용도입니다.
- 콘텐츠의 다운로드, 저장, 변환, 사용이 권리자의 허가를 받았고 적용되는 법률 및 대상 사이트/플랫폼의 서비스 약관을 준수하는지 확인할 책임은 사용자에게 있습니다.
- 침해 콘텐츠, 무단 콘텐츠, 유료 또는 제한 콘텐츠, 개인정보와 관련된 콘텐츠, 기타 불법 콘텐츠를 다운로드, 배포, 판매하거나 이용하기 위해 XiaDown을 사용하지 마세요.
- XiaDown 사용으로 발생하는 저작권, 플랫폼 정책, 계정, 네트워크 또는 기타 법적 책임은 사용자 본인에게 있으며, 프로젝트 관리자는 사용자의 행위와 그 결과에 책임을 지지 않습니다.

## 감사의 글

XiaDown은 훌륭한 오픈 소스 프로젝트 위에 만들어졌습니다. 데스크톱 경험, 미디어 처리, 로컬 저장소, 브라우저 연결, 온라인 음악, 프론트엔드 인터페이스는 모두 이러한 기반에 의존합니다.

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

- 웹사이트: <https://xiadown.dreamapp.cc/>
- 이메일: <xunruhao@gmail.com>
