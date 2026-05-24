<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Biểu tượng XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Một công cụ tải video có hỗ trợ nhạc trực tuyến.</strong></p>
  <p>Listen Keep, Make it Yours</p>
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
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Phiên bản mới nhất" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Giấy phép" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Nền tảng được hỗ trợ" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Stack công nghệ" />
  </p>
</div>

## Tổng Quan

XiaDown là trình phát nhạc trực tuyến và cũng là công cụ tải video.

Ứng dụng được tạo cho nhà sáng tạo nội dung: khi cần tư liệu, XiaDown cung cấp khả năng tải mạnh mẽ dựa trên YT-DLP; khi cần làm việc, nhạc trực tuyến có thể phát trong nền. Nhờ pet và giao diện có thể tùy chỉnh, ứng dụng vẫn đơn giản nhưng không nhàm chán.

## Tính Năng Chính

- **Trình phát nhạc trực tuyến**: trình phát desktop dành cho các kênh YouTube Lo-Fi và YouTube Music, hỗ trợ đăng nhập tài khoản, tìm kiếm bài hát, nghệ sĩ và playlist, hàng đợi phát, lời bài hát, ảnh bìa và thêm kênh Lo-Fi trực tuyến tùy chỉnh. Những bản nhạc muốn giữ lâu dài có thể được tải về thư viện cục bộ.
- **Tải video và âm thanh**: được hỗ trợ bởi YT-DLP, có thể tải tư liệu từ hàng nghìn trang video trực tuyến; dán liên kết để lưu video, âm thanh, phụ đề và ảnh bìa, sau đó chuyển mã và quản lý trong thư viện cục bộ.
- **Không gian media cá nhân hóa**: các gói chủ đề được thiết kế kỹ, màu nhấn, chế độ giao diện, kiểu sidebar và hỗ trợ đầy đủ Codex Pets, cùng các phụ thuộc và cập nhật ứng dụng được duy trì tự động cho việc sử dụng hằng ngày lâu dài.

## Xem Trước Sản Phẩm

<p align="center">
  <img src="./images/download.png" alt="Màn hình tác vụ tải xuống của XiaDown" width="88%" />
</p>

<p align="center">
  <img src="./images/listen.png" alt="Màn hình phát nhạc trực tuyến XiaDown Listen" width="88%" />
</p>

<p align="center">
  <img src="./images/library.png" alt="Màn hình thư viện của XiaDown" width="88%" />
</p>

## Bắt Đầu Nhanh

### Tải xuống và cài đặt

Tải trực tiếp gói mới nhất bên dưới. Các phiên bản cũ có trên [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Nền tảng | Kiến trúc | Gói | Tải xuống |
| --- | --- | --- | --- |
| macOS | Apple Silicon | Tệp nén | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | Tệp nén | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | Bộ cài | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Bản portable | [Tải xuống](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### Lần chạy đầu tiên

1. `macOS`: giải nén gói và chuyển `XiaDown.app` vào thư mục Applications. Nếu macOS báo ứng dụng không thể mở hoặc bị hỏng, hãy chạy `sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app` trong terminal.
2. `Windows`: chạy trực tiếp bộ cài `.exe`, hoặc giải nén gói portable rồi mở ứng dụng. Nếu SmartScreen xuất hiện trong lần chạy đầu tiên, chọn `More info -> Run anyway`.
3. XiaDown mở luồng onboarding để thiết lập ngôn ngữ, chủ đề, proxy và các phụ thuộc. Các luồng chính nằm trong onboarding và giao diện ứng dụng.

## Tuyên Bố Miễn Trừ Trách Nhiệm

- XiaDown được cung cấp như một công cụ hỗ trợ quản lý phương tiện và tải xuống, phục vụ học tập, nghiên cứu và lưu nội dung mà bạn có quyền truy cập và sử dụng.
- Bạn chịu trách nhiệm xác nhận rằng mọi việc tải xuống, lưu trữ, chuyển đổi hoặc sử dụng nội dung đã được chủ sở hữu quyền cho phép và tuân thủ pháp luật hiện hành cũng như điều khoản của website/nền tảng đích.
- Không sử dụng XiaDown để tải xuống, phân phối, bán hoặc khai thác nội dung vi phạm, chưa được cấp phép, trả phí/bị giới hạn, riêng tư hoặc bất hợp pháp.
- Mọi trách nhiệm pháp lý liên quan đến bản quyền, quy định nền tảng, tài khoản, mạng hoặc vấn đề khác phát sinh từ việc sử dụng XiaDown thuộc về người dùng; người duy trì dự án không chịu trách nhiệm đối với hành vi của người dùng hoặc hậu quả của hành vi đó.

## Lời Cảm Ơn

XiaDown được xây dựng trên các dự án mã nguồn mở xuất sắc. Trải nghiệm desktop, xử lý media, lưu trữ cục bộ, kết nối trình duyệt, nhạc trực tuyến và giao diện frontend đều dựa trên những nền tảng này.

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

- Trang web: <https://xiadown.dreamapp.cc/>
- Email: <xunruhao@gmail.com>
