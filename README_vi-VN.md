<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Biểu tượng XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Một công cụ tải video hai cơ chế, hỗ trợ nhạc trực tuyến.</strong></p>
  <p>Listen Keep, Make it Yours</p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Phiên bản mới nhất" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Giấy phép" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Nền tảng được hỗ trợ" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Stack công nghệ" />
  </p>
  <p>
    <a href="https://xiadown.app/">Trang web</a> ·
    <a href="https://xiadown.app/vi-vn/docs/">Hướng dẫn sử dụng</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Bản phát hành</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Issues</a> ·
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
  <img src="./images/download.webp" alt="Màn hình tác vụ tải xuống của XiaDown" width="92%" />
  <br />
  <sub>Tác vụ tải xuống và chuyển mã</sub>
</p>

## Tổng Quan

XiaDown là trình phát nhạc trực tuyến, đồng thời là công cụ tải video với hai cơ chế tải.

Đây là công cụ hằng ngày dành cho nhà sáng tạo nội dung: khi cần tư liệu, hãy tải bằng bắt tài nguyên và YT-DLP; khi cần tập trung, hãy để nhạc trực tuyến phát trong nền; các tùy chọn cá nhân hóa phong phú giúp việc sử dụng lâu dài vẫn nhẹ nhàng và mới mẻ.

## Tính Năng Chính

### 📥 Tải Xuống và Chuyển Mã

- **Tải bằng bắt tài nguyên**: quan sát video, âm thanh, live stream, manifest, hình ảnh, phụ đề và phản hồi API qua CDP; phù hợp với các site như TikTok, Douyin, Kuaishou và Xiaohongshu cần phiên trình duyệt thật.
- **Tải bằng YT-DLP**: dán liên kết để phân tích các nền tảng phổ biến như YouTube và Bilibili, lưu video, âm thanh, phụ đề và ảnh bìa, đồng thời dùng danh tính đã đăng nhập để tải nội dung mà bạn có quyền truy cập.
- **Chuyển mã âm thanh và video**: dựa trên FFmpeg, hỗ trợ chuyển mã sau khi tải xuống và chuyển mã file cục bộ, với các preset tích hợp như H.264, H.265, VP9, MP3, AAC, Opus, FLAC và WAV.

### 🗂️ Quản Lý Tài Nguyên

- **Quản lý tài nguyên đa chế độ xem**: chế độ xem tác vụ và chế độ xem file thống nhất tải xuống, chuyển mã, phụ đề, ảnh bìa và file đã nhập, kèm xem trước, chi tiết, chọn hàng loạt, xóa, khôi phục lỗi và dọn bản ghi cũ.

### 🎧 Trình Phát

- **Phát nhạc cục bộ**: tự động lập chỉ mục âm thanh trong thư viện, hỗ trợ hàng đợi, ảnh bìa, lời bài hát đồng bộ, lời roman hóa/pinyin cho ngôn ngữ Đông Á, equalizer và hiển thị phổ âm.
- **YouTube Music**: tìm bài hát, nghệ sĩ và playlist trong trải nghiệm desktop, với đề xuất trang chủ, thư viện playlist, nghệ sĩ theo dõi, nhạc đã thích, hàng đợi phát và lời bài hát.
- **YouTube Live**: tạo nhóm và kênh live tùy chỉnh, xem trạng thái live, phát radio live và mở trực tiếp video live.

### 🔐 An Toàn và Cách Ly

- **Tự động quản lý phụ thuộc**: tự động cài đặt, kiểm tra và nâng cấp YT-DLP, FFmpeg, Bun cùng các công cụ liên quan; đường dẫn công cụ do ứng dụng duy trì và không làm bẩn môi trường hệ thống.
- **Cách ly thông tin xác thực và người dùng**: dữ liệu phiên ứng dụng đến từ thao tác đăng nhập chủ động của người dùng và được lưu trữ độc lập bằng cơ chế mã hóa hệ thống trên macOS và Windows; cấu hình kết nối tách khỏi trình duyệt dùng hằng ngày.

### 🎨 Tự Do Tùy Biến

- **Tùy biến giao diện**: hỗ trợ gói chủ đề, chế độ sáng/tối/tự động, màu nhấn, font, cỡ chữ và kiểu sidebar; Codex Pets Gallery tích hợp có thể nhập Pets online và cục bộ.

## Xem Trước Sản Phẩm

<p align="center">
  <img src="./images/sniff-desk.webp" alt="Màn hình bắt tài nguyên của XiaDown" width="92%" />
  <br />
  <sub>Bảng bắt tài nguyên từ trang web</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="Màn hình phát YouTube Music trong XiaDown" width="92%" />
  <br />
  <sub>Phát YouTube Music trên desktop</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="Màn hình thư viện của XiaDown" width="92%" />
  <br />
  <sub>Thư viện thống nhất cho nội dung đã tải và đã chuyển mã</sub>
</p>

<details>
  <summary><strong>Thêm ảnh chụp giao diện</strong></summary>

  <p align="center">
    <img src="./images/youtube-live.webp" alt="Màn hình video YouTube Live trong XiaDown" width="92%" />
    <br />
    <sub>Xem video YouTube Live</sub>
  </p>

  <p align="center">
    <img src="./images/app_sessions.webp" alt="Màn hình phiên ứng dụng và trạng thái đăng nhập của XiaDown" width="92%" />
    <br />
    <sub>Phiên ứng dụng, xác minh đăng nhập và quản lý trạng thái tài khoản</sub>
  </p>

  <p align="center">
    <img src="./images/settings_download.webp" alt="Màn hình cài đặt tải xuống và quản lý công cụ phụ thuộc của XiaDown" width="92%" />
    <br />
    <sub>Thư mục tải xuống, số tác vụ đồng thời và quản lý YT-DLP, FFmpeg, Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="Màn hình cài đặt giao diện của XiaDown" width="92%" />
    <br />
    <sub>Chủ đề, chế độ sáng/tối, màu nhấn, font và cỡ chữ</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="Màn hình Codex Pets Gallery trong XiaDown" width="92%" />
    <br />
    <sub>Codex Pets Gallery và nhập Pet cục bộ</sub>
  </p>
</details>

## Cài Đặt

### Homebrew

Trên macOS, cài bằng Homebrew cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Tải bộ cài

Tải trực tiếp gói mới nhất bên dưới. Các phiên bản cũ có trên [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Nền tảng | Kiến trúc | Gói | Tải xuống |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Bộ cài | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Bản portable | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### Lần chạy đầu tiên

1. `macOS`: mở `.dmg`, kéo `XiaDown.app` vào thư mục Applications rồi mở ứng dụng.
2. `Windows`: chạy trực tiếp bộ cài `.exe`, hoặc giải nén gói portable rồi mở ứng dụng. Nếu SmartScreen xuất hiện trong lần chạy đầu tiên, chọn `More info -> Run anyway`.
3. XiaDown mở luồng onboarding để thiết lập ngôn ngữ, chủ đề, proxy và các phụ thuộc. Các luồng chính nằm trong onboarding và giao diện ứng dụng.

### Hỗ Trợ Trình Duyệt CDP

Các trình duyệt hiện được hỗ trợ:

| Phổ biến | Quyền riêng tư và hiệu quả | Chuyên biệt và khu vực |
| --- | --- | --- |
| Chrome, Chromium, Edge | Brave, Vivaldi, Arc, Helium | Opera, Opera GX, Yandex Browser |

## Phát triển cục bộ

Sau khi chuẩn bị môi trường Go và Bun, cài Wails 3 CLI rồi khởi động chế độ phát triển:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.95
wails3 dev
```

## Tuyên Bố Miễn Trừ Trách Nhiệm

- XiaDown được cung cấp như một công cụ hỗ trợ quản lý phương tiện và tải xuống, phục vụ học tập, nghiên cứu và lưu nội dung mà bạn có quyền truy cập và sử dụng.
- Bạn chịu trách nhiệm xác nhận rằng mọi việc tải xuống, lưu trữ, chuyển đổi hoặc sử dụng nội dung đã được chủ sở hữu quyền cho phép và tuân thủ pháp luật hiện hành cũng như điều khoản của website/nền tảng đích.
- Không sử dụng XiaDown để tải xuống, phân phối, bán hoặc khai thác nội dung vi phạm, chưa được cấp phép, trả phí/bị giới hạn, riêng tư hoặc bất hợp pháp.
- Mọi trách nhiệm pháp lý liên quan đến bản quyền, quy định nền tảng, tài khoản, mạng hoặc vấn đề khác phát sinh từ việc sử dụng XiaDown thuộc về người dùng; người duy trì dự án không chịu trách nhiệm đối với hành vi của người dùng hoặc hậu quả của hành vi đó.

## Lời Cảm Ơn

XiaDown được xây dựng trên các dự án mã nguồn mở xuất sắc. Trải nghiệm desktop, xử lý media, lưu trữ cục bộ, kết nối trình duyệt, nhạc trực tuyến và năng lực giao diện đều dựa trên những nền tảng này.

| Danh mục | Trang chủ |
| --- | --- |
| Framework desktop | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| Xử lý media | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| Lưu trữ cục bộ | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| Kết nối trình duyệt | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| Trải nghiệm frontend | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## Hợp Tác

- Dự án đang được phát triển tích cực và hiện chưa nhận pull request. Phản hồi, báo lỗi và kịch bản sử dụng có thể gửi qua [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) hoặc email.
- Kho này được cấp phép theo `Apache-2.0`. Xem [LICENSE](./LICENSE).

## Liên Hệ

- Trang web: <https://xiadown.app/>
- Hướng dẫn sử dụng: <https://xiadown.app/vi-vn/docs/>
- Email: <xunruhao@gmail.com>
