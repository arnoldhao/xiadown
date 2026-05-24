<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ikon XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Alat pengunduh video dengan dukungan musik online.</strong></p>
  <p>Listen Keep, Make it Yours</p>
  <p>
    <a href="./README.md">简体中文</a> ·
    <a href="./README_zh-Hant.md">繁體中文</a> ·
    <a href="./README_en.md">English</a> ·
    <a href="./README_ja-JP.md">日本語</a> ·
    <a href="./README_ko-KR.md">한국어</a> ·
    <a href="./README_es-419.md">Español (LatAm)</a> ·
    <a href="./README_pt-BR.md">Português (BR)</a> ·
    <strong>Bahasa Indonesia</strong> ·
    <a href="./README_vi-VN.md">Tiếng Việt</a>
  </p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Versi terbaru" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Lisensi" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Platform yang didukung" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Stack teknologi" />
  </p>
</div>

## Gambaran Umum

XiaDown adalah pemutar musik online sekaligus alat pengunduh video.

Aplikasi ini dibuat untuk kreator konten: saat Anda membutuhkan materi, XiaDown menyediakan kemampuan unduh yang kuat berbasis YT-DLP; saat Anda bekerja, musik online dapat tetap diputar di latar belakang. Dengan pet dan tampilan yang bisa disesuaikan, aplikasi tetap sederhana tanpa terasa membosankan.

## Kemampuan Utama

- **Pemutar musik online**: pemutar desktop yang dirancang untuk stasiun YouTube Lo-Fi dan YouTube Music, dengan login akun, pencarian lagu, artis, dan playlist, antrean pemutaran, lirik, artwork, serta dukungan untuk stasiun Lo-Fi online kustom. Trek yang ingin disimpan dapat diunduh ke pustaka lokal.
- **Unduhan video dan audio**: didukung oleh YT-DLP, dengan dukungan unduh materi dari ribuan situs video online; tempel tautan untuk menyimpan video, audio, subtitle, dan sampul, lalu lakukan transcoding dan kelola semuanya di pustaka lokal.
- **Ruang media yang dipersonalisasi**: paket tema yang dirancang dengan cermat, warna aksen, mode tampilan, gaya sidebar, dan dukungan penuh Codex Pets, dengan dependensi dan pembaruan aplikasi yang dikelola otomatis untuk penggunaan harian jangka panjang.

## Pratinjau Produk

<p align="center">
  <img src="./images/download.png" alt="Tampilan tugas unduhan XiaDown" width="88%" />
</p>

<p align="center">
  <img src="./images/listen.png" alt="Tampilan pemutaran musik online XiaDown Listen" width="88%" />
</p>

<p align="center">
  <img src="./images/library.png" alt="Tampilan pustaka XiaDown" width="88%" />
</p>

## Mulai Cepat

### Unduh dan instal

Unduh paket terbaru secara langsung di bawah ini. Versi lama tersedia di [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Platform | Arsitektur | Paket | Unduh |
| --- | --- | --- | --- |
| macOS | Apple Silicon | Arsip | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | Arsip | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | Installer | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portabel | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### Peluncuran pertama

1. `macOS`: ekstrak paket dan pindahkan `XiaDown.app` ke folder Applications. Jika macOS mengatakan aplikasi tidak dapat dibuka atau rusak, jalankan `sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app` di terminal.
2. `Windows`: jalankan installer `.exe` secara langsung, atau ekstrak paket portabel dan buka aplikasinya. Jika SmartScreen muncul saat pertama kali dibuka, pilih `More info -> Run anyway`.
3. XiaDown membuka alur onboarding untuk pengaturan bahasa, tema, proxy, dan dependensi. Alur utama berada di onboarding dan UI aplikasi.

## Penafian

- XiaDown disediakan sebagai alat bantu manajemen media dan pengunduhan, untuk pembelajaran, riset, dan penyimpanan konten yang memang berhak Anda akses dan gunakan.
- Anda bertanggung jawab untuk memastikan setiap pengunduhan, penyimpanan, konversi, atau penggunaan konten telah diizinkan oleh pemegang hak dan mematuhi hukum yang berlaku serta ketentuan situs/platform tujuan.
- Jangan gunakan XiaDown untuk mengunduh, mendistribusikan, menjual, atau mengeksploitasi konten yang melanggar hak, tidak berizin, berbayar/terbatas, bersifat privat, atau melanggar hukum.
- Setiap tanggung jawab hukum terkait hak cipta, kebijakan platform, akun, jaringan, atau hal lain yang timbul dari penggunaan XiaDown menjadi tanggung jawab pengguna; pengelola proyek tidak bertanggung jawab atas tindakan pengguna maupun konsekuensinya.

## Ucapan Terima Kasih

XiaDown dibangun di atas proyek open source yang sangat baik. Pengalaman desktop, pipeline media, penyimpanan lokal, koneksi browser, musik online, dan antarmuka frontend semuanya bergantung pada fondasi ini.

| Kategori | Beranda |
| --- | --- |
| Framework desktop | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| Pemrosesan media | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| Penyimpanan lokal | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| Koneksi browser | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| Pengalaman frontend | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## Kolaborasi

- Proyek ini sedang aktif dikembangkan dan untuk saat ini belum menerima pull request. Masukan, laporan bug, dan skenario penggunaan dapat dikirim melalui [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) atau email.
- Repositori ini dilisensikan di bawah `Apache-2.0`. Lihat [LICENSE](./LICENSE).

## Kontak

- Situs web: <https://xiadown.dreamapp.cc/>
- Email: <xunruhao@gmail.com>
