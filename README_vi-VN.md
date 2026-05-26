<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Biểu tượng XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Một công cụ tải video hai cơ chế, hỗ trợ nhạc trực tuyến.</strong></p>
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

XiaDown là trình phát nhạc trực tuyến, đồng thời là công cụ tải video với hai cơ chế tải.

Ứng dụng được tạo cho nhà sáng tạo nội dung: khi cần tư liệu, XiaDown cung cấp khả năng tải mạnh mẽ bằng bắt tài nguyên trình duyệt và YT-DLP; khi cần làm việc, nhạc trực tuyến có thể phát trong nền. Với thư viện, chuyển mã, tự động quản lý phụ thuộc, cách ly tài khoản, pet và tùy biến giao diện, XiaDown vừa xử lý tư liệu media, vừa có thể trở thành công cụ media desktop dùng hằng ngày.

## Tính Năng Chính

- 📥 **Tải video hai cơ chế**: tích hợp engine tải YT-DLP và engine bắt tài nguyên trình duyệt dựa trên CDP. Liên kết thông thường có thể được phân tích và tải trực tiếp, kể cả khi dùng Cookies đã lưu trong hồ sơ kết nối; với trang tải động, cấu trúc site phức tạp hoặc tài nguyên cần phiên trình duyệt thật, chế độ bắt tài nguyên có thể nhận diện video, âm thanh, phụ đề, ảnh bìa và các tài nguyên media khác, rồi tiếp tục chuyển mã và quản lý trong thư viện.
- 🎧 **Trình phát nhạc desktop**: tự động quản lý âm thanh cục bộ trong các tài nguyên đã tải, hỗ trợ tìm kiếm bài hát, nghệ sĩ và playlist trên YouTube Music, cũng như phát radio YouTube Live và xem video live. Trình phát có hàng đợi, ảnh bìa, lời bài hát đồng bộ, lời roman hóa/pinyin cho ngôn ngữ Đông Á, equalizer và hiển thị phổ âm.
- 🧩 **Không gian sử dụng tự do và có kiểm soát**: công cụ phụ thuộc được ứng dụng tự cài đặt và nâng cấp, không làm bẩn môi trường hệ thống; tài khoản, Cookies và Profile trình duyệt được quản lý bằng cấu hình kết nối tách biệt; chủ đề, chế độ sáng/tối, màu nhấn, font, cỡ chữ và Codex Pets đều có thể tùy chỉnh.

## Khả Năng Cốt Lõi

- **Tải bằng bắt tài nguyên**: năng lực bắt tài nguyên trình duyệt tự phát triển dựa trên CDP, có thể quan sát video, âm thanh, live stream, manifest, hình ảnh, phụ đề, phản hồi API và các tài nguyên khác trên trang. Trong môi trường trình duyệt thật sau khi người dùng chủ động đăng nhập, XiaDown có thể nhận diện và tải tài nguyên từ TikTok, Douyin, Kuaishou, Xiaohongshu và các site tương tự, đồng thời liên kết tải xuống trực tiếp với quy trình chuyển mã.
- **Tải bằng YT-DLP**: tích hợp YT-DLP để tải tư liệu từ nhiều website video trực tuyến, hỗ trợ ổn định các nền tảng phổ biến như YouTube và Bilibili. Chỉ cần dán liên kết để phân tích và lưu video, âm thanh, phụ đề, ảnh bìa; cũng có thể dùng Cookies đã lưu trong hồ sơ kết nối để tải nội dung mà người dùng có quyền truy cập, rồi tiếp tục chuyển mã và quản lý trong thư viện.
- **Chuyển mã âm thanh và video**: dựa trên FFmpeg, hỗ trợ chuyển mã sau khi tải xuống hoặc chọn thủ công file cục bộ. Các preset tích hợp gồm H.264, H.265, VP9, MP3, AAC, Opus, FLAC, WAV và các đầu ra thường dùng như kích thước gốc, 2160p, 1080p, 720p, 480p.
- **Quản lý tài nguyên đa chế độ xem**: chế độ xem tác vụ và chế độ xem file thống nhất tải xuống, chuyển mã, phụ đề, ảnh bìa và file đã nhập. Hỗ trợ xem trước media, chi tiết tác vụ, chi tiết file, chọn hàng loạt, xóa, khôi phục tác vụ thất bại, kiểm tra sự tồn tại của file và dọn các bản ghi không còn hợp lệ.
- **Phát nhạc cục bộ**: tự động lập chỉ mục file âm thanh trong thư viện, hỗ trợ phát cục bộ, hàng đợi phát, hiển thị ảnh bìa, lời bài hát đồng bộ, lời roman hóa/pinyin cho ngôn ngữ Đông Á, equalizer và nhiều kiểu hiển thị phổ âm.
- **YouTube Music**: cung cấp trải nghiệm YouTube Music trên desktop, hỗ trợ kết nối tài khoản, tìm kiếm bài hát/nghệ sĩ/playlist, đề xuất trang chủ, thư viện playlist, nghệ sĩ theo dõi, nhạc đã thích, hàng đợi phát, lời bài hát và dọn dữ liệu quảng cáo để giảm gián đoạn khi phát.
- **YouTube Live**: hỗ trợ tự thêm nhóm và kênh YouTube Live, xem trạng thái live, phát radio live và xem trực tiếp video live.
- **Tự động quản lý phụ thuộc**: tự động duy trì cài đặt, kiểm tra và nâng cấp YT-DLP, FFmpeg, Bun cùng các công cụ liên quan. Đường dẫn công cụ do ứng dụng quản lý độc lập, không phụ thuộc và không làm bẩn môi trường global của người dùng.
- **Cách ly thông tin xác thực và người dùng**: hỗ trợ gọi năng lực trình duyệt cục bộ qua CDP và lưu bền vững Profiles cùng Cookies độc lập. Dữ liệu chỉ đến từ thao tác đăng nhập chủ động của người dùng, còn cấu hình kết nối được tách khỏi môi trường trình duyệt hằng ngày.
- **Tùy biến giao diện**: hỗ trợ gói chủ đề, chế độ sáng/tối/tự động, màu nhấn, font, cỡ chữ, kiểu sidebar và nhiều thiết lập giao diện khác. Codex Pets Gallery tích hợp có thể nhập Pets online và cục bộ làm phần tử đồng hành trên desktop.

## Xem Trước Sản Phẩm

<p align="center">
  <img src="./images/download.webp" alt="Màn hình tác vụ tải xuống của XiaDown" width="88%" />
  <br />
  <sub>Tác vụ tải xuống và chuyển mã</sub>
</p>

<p align="center">
  <img src="./images/sniff-desk.webp" alt="Màn hình bắt tài nguyên của XiaDown" width="88%" />
  <br />
  <sub>Bảng bắt tài nguyên từ trang web</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="Màn hình phát YouTube Music trong XiaDown" width="88%" />
  <br />
  <sub>Phát YouTube Music trên desktop</sub>
</p>

<p align="center">
  <img src="./images/youtube-live.webp" alt="Màn hình video YouTube Live trong XiaDown" width="88%" />
  <br />
  <sub>Xem video YouTube Live</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="Màn hình thư viện của XiaDown" width="88%" />
  <br />
  <sub>Thư viện thống nhất cho nội dung đã tải và đã chuyển mã</sub>
</p>

<details>
  <summary>Thêm màn hình cài đặt và cá nhân hóa</summary>

  <p align="center">
    <img src="./images/connector.webp" alt="Màn hình kết nối và cách ly tài khoản của XiaDown" width="88%" />
    <br />
    <sub>Cách ly cấu hình kết nối, Cookies và Profile trình duyệt</sub>
  </p>

  <p align="center">
    <img src="./images/tools.webp" alt="Màn hình quản lý công cụ phụ thuộc của XiaDown" width="88%" />
    <br />
    <sub>Tự động quản lý YT-DLP, FFmpeg và Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="Màn hình cài đặt giao diện của XiaDown" width="88%" />
    <br />
    <sub>Chủ đề, chế độ sáng/tối, màu nhấn, font và cỡ chữ</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="Màn hình Codex Pets Gallery trong XiaDown" width="88%" />
    <br />
    <sub>Codex Pets Gallery và nhập Pet cục bộ</sub>
  </p>
</details>

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
