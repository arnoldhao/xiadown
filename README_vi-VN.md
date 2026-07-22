<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Biểu tượng XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Ứng dụng quản lý thư viện nội dung hỗ trợ tải video</strong></p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Phiên bản mới nhất" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Giấy phép" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Nền tảng được hỗ trợ" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Công nghệ" />
  </p>
  <p>
    <a href="https://xiadown.app/">Trang chủ</a> ·
    <a href="https://xiadown.app/docs/">Tài liệu</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Bản phát hành</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Báo lỗi</a> ·
    <a href="https://ko-fi.com/arnoldhao">Ủng hộ</a>
  </p>
  <p>
    <a href="./README.md">简体中文</a> ·
    <a href="./README_zh-Hant.md">繁體中文</a> ·
    <a href="./README_en.md">English</a> ·
    <a href="./README_ja-JP.md">日本語</a> ·
    <a href="./README_ko-KR.md">한국어</a> ·
    <a href="./README_es-419.md">Español (LatAm)</a> ·
    <a href="./README_pt-BR.md">Português (BR)</a> ·
    <a href="./README_id-ID.md">Bahasa Indonesia</a> ·
    <strong>Tiếng Việt</strong>
  </p>
</div>

<p align="center">
  <a href="https://xiadown.app/docs/library/">
    <img src="./images/library.webp" alt="Giao diện thư viện XiaDown" width="92%" />
  </a>
  <br />
  <strong>Thư viện</strong>
</p>

## Giới thiệu dự án

XiaDown là ứng dụng quản lý thư viện ưu tiên lưu trữ cục bộ, hỗ trợ tải video và dò tìm nội dung đa phương tiện để tải xuống. XiaDown còn là ứng dụng khách dành cho YouTube, YouTube Music và RSS. Khi duyệt nội dung, bạn có thể tải nội dung mình cần chỉ với một cú nhấp.

## Khả năng chính

- 🗂️ **[Thư viện](https://xiadown.app/docs/library/)** — Tải video, chuyển mã và quản lý thư viện.
- 🔎 **[Dò tìm](https://xiadown.app/docs/sniff/)** — Dò tìm và tải nội dung đa phương tiện trên trang web.
- 🎵 **[Âm nhạc](https://xiadown.app/docs/music/)** — Duyệt và tải nội dung từ YouTube Music, đồng thời phát nhạc cục bộ.
- ▶️ **[YouTube](https://xiadown.app/docs/youtube/)** — Duyệt, phát và tải video.
- 📡 **[RSS](https://xiadown.app/docs/rss/)** — Đăng ký nguồn tin, đọc bài viết và tải nội dung đa phương tiện.

## Ứng dụng di động

📱 Ứng dụng dành cho iPhone và iPad đang được phát triển.

## Giao diện sản phẩm

<table>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/sniff/">
        <img src="./images/sniff-desk.webp" alt="Giao diện dò tìm của XiaDown" width="100%" />
      </a>
      <br />
      <strong>Dò tìm</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/rss/">
        <img src="./images/rss.webp" alt="Giao diện đăng ký RSS của XiaDown" width="100%" />
      </a>
      <br />
      <strong>RSS</strong>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/music/">
        <img src="./images/youtube-music.webp" alt="Giao diện âm nhạc của XiaDown" width="100%" />
      </a>
      <br />
      <strong>Âm nhạc</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/youtube/">
        <img src="./images/youtube-live.webp" alt="Giao diện duyệt video YouTube của XiaDown" width="100%" />
      </a>
      <br />
      <strong>YouTube</strong>
    </td>
  </tr>
</table>

## Cài đặt

### Homebrew

Trên macOS, cài đặt qua Homebrew cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Tải bộ cài

| Nền tảng | Kiến trúc | Định dạng | Tải xuống |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Bộ cài | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Bản portable | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

> Phiên bản macOS yêu cầu macOS 14 (Sonoma) trở lên; phiên bản Windows yêu cầu Windows 10 trở lên.

Trong lần đầu khởi chạy, XiaDown sẽ hướng dẫn bạn thiết lập ngôn ngữ, giao diện, mạng và các phần phụ thuộc cần thiết để vận hành. Xem [Cài đặt và khởi chạy lần đầu](https://xiadown.app/docs/start/install/) để biết hướng dẫn chi tiết.

## Phát triển cục bộ

Môi trường phát triển yêu cầu Go 1.25.12, Node.js 24, Bun 1.3.5 và Wails 3 alpha2.117:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
wails3 task dev
```

Các tác vụ build và kiểm tra khác có trong [Taskfile.yml](./Taskfile.yml).

## Tuyên bố miễn trừ trách nhiệm

- XiaDown chỉ dùng để quản lý nội dung đa phương tiện và lưu nội dung mà bạn có quyền truy cập và sử dụng.
- Người dùng có trách nhiệm xác nhận việc tải xuống, lưu trữ, chuyển đổi và sử dụng nội dung tuân thủ pháp luật sở tại, sự cho phép của chủ sở hữu quyền và điều khoản dịch vụ của nền tảng đích.
- Không sử dụng XiaDown để xử lý nội dung vi phạm quyền của bên khác, chưa được cấp phép, trả phí hoặc bị hạn chế, liên quan đến quyền riêng tư hay trái pháp luật.
- Người dùng tự chịu mọi trách nhiệm về bản quyền, quy định nền tảng, tài khoản, mạng và các vấn đề khác phát sinh từ việc sử dụng XiaDown.

## Lời cảm ơn

XiaDown sử dụng các dự án mã nguồn mở như [Go](https://go.dev/), [Wails](https://v3alpha.wails.io/), [React](https://react.dev/), [yt-dlp](https://github.com/yt-dlp/yt-dlp), [FFmpeg](https://ffmpeg.org/) và [SQLite](https://www.sqlite.org/). Thông tin về thành phần phụ thuộc và giấy phép có trong [THIRD_PARTY_NOTICES.txt](./frontend/public/THIRD_PARTY_NOTICES.txt).

## Cộng tác

- Dự án hiện chưa tiếp nhận pull request. Hãy gửi báo cáo lỗi và đề xuất qua [GitHub Issues](https://github.com/arnoldhao/xiadown/issues).
- Kho mã được cấp phép theo `Apache-2.0`. Xem [LICENSE](./LICENSE).

## Liên hệ

- Trang chủ: <https://xiadown.app/>
- Tài liệu: <https://xiadown.app/docs/>
- Email: <xunruhao@gmail.com>
