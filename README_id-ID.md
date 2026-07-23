<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ikon XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Aplikasi pengelola pustaka media yang mendukung pengunduhan video</strong></p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Versi terbaru" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Lisensi" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Platform yang didukung" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Teknologi" />
  </p>
  <p>
    <a href="https://xiadown.app/">Situs web</a> ·
    <a href="https://xiadown.app/docs/">Dokumentasi</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Rilis</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Laporkan masalah</a> ·
    <a href="https://ko-fi.com/arnoldhao">Dukung</a>
  </p>
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
</div>

<p align="center">
  <a href="https://xiadown.app/docs/library/">
    <img src="./images/library.webp" alt="Tampilan pustaka XiaDown" width="92%" />
  </a>
  <br />
  <strong>Pustaka</strong>
</p>

## Tentang Proyek

XiaDown adalah aplikasi pengelola pustaka yang mengutamakan penyimpanan lokal serta mendukung pengunduhan video dan media yang terdeteksi. XiaDown juga berfungsi sebagai klien YouTube, YouTube Music, dan RSS. Saat menjelajah, Anda dapat mengunduh media yang diperlukan dengan sekali klik.

## Kemampuan Utama

- 🗂️ **[Pustaka](https://xiadown.app/docs/library/)** — Pengunduhan dan transkode video serta pengelolaan pustaka.
- 🔎 **[Deteksi Media](https://xiadown.app/docs/sniff/)** — Mendeteksi dan mengunduh media dari halaman web.
- 🎵 **[Musik](https://xiadown.app/docs/music/)** — Menjelajahi dan mengunduh konten dari YouTube Music serta memutar musik lokal.
- ▶️ **[YouTube](https://xiadown.app/docs/youtube/)** — Menjelajah, memutar, dan mengunduh video.
- 📡 **[RSS](https://xiadown.app/docs/rss/)** — Berlangganan dan membaca konten serta mengunduh media.

## Aplikasi Seluler

📱 Klien untuk iPhone dan iPad sedang dikembangkan.

## Tampilan Produk

<table>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/sniff/">
        <img src="./images/sniff-desk.webp" alt="Tampilan deteksi media XiaDown" width="100%" />
      </a>
      <br />
      <strong>Deteksi Media</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/rss/">
        <img src="./images/rss.webp" alt="Tampilan langganan RSS XiaDown" width="100%" />
      </a>
      <br />
      <strong>RSS</strong>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/music/">
        <img src="./images/youtube-music.webp" alt="Tampilan musik XiaDown" width="100%" />
      </a>
      <br />
      <strong>Musik</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/youtube/">
        <img src="./images/youtube-live.webp" alt="Tampilan penjelajahan video YouTube di XiaDown" width="100%" />
      </a>
      <br />
      <strong>YouTube</strong>
    </td>
  </tr>
</table>

## Instalasi

### Homebrew

Di macOS, instal melalui Homebrew cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Unduh Paket Instalasi

| Platform | Arsitektur | Format | Unduh |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Installer | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portabel | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

> Versi macOS memerlukan macOS 14 (Sonoma) atau lebih baru; versi Windows memerlukan Windows 10 atau lebih baru.

Saat pertama kali dijalankan, XiaDown akan memandu Anda mengatur bahasa, tampilan, jaringan, dan dependensi runtime. Lihat [Instalasi dan Peluncuran Pertama](https://xiadown.app/docs/start/install/) untuk petunjuk lengkap.

## Pengembangan Lokal

Lingkungan pengembangan memerlukan Go 1.25.12, Node.js 24, Bun 1.3.5, dan Wails 3 alpha2.117:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
wails3 task dev
```

Tugas build dan pemeriksaan lainnya tersedia di [Taskfile.yml](./Taskfile.yml).

## Penafian

- XiaDown hanya ditujukan untuk mengelola media dan menyimpan konten yang berhak Anda akses dan gunakan.
- Pengguna bertanggung jawab memastikan bahwa pengunduhan, penyimpanan, konversi, dan penggunaan konten mematuhi hukum setempat, izin pemegang hak, serta ketentuan layanan platform tujuan.
- Jangan gunakan XiaDown untuk memproses konten yang melanggar hak pihak lain, tanpa izin, berbayar atau dibatasi, bersifat privat, maupun melanggar hukum.
- Seluruh tanggung jawab terkait hak cipta, aturan platform, akun, jaringan, dan hal lain yang timbul dari penggunaan XiaDown berada pada pengguna.

## Ucapan Terima Kasih

XiaDown menggunakan berbagai proyek sumber terbuka, termasuk [Go](https://go.dev/), [Wails](https://v3alpha.wails.io/), [React](https://react.dev/), [yt-dlp](https://github.com/yt-dlp/yt-dlp), [FFmpeg](https://ffmpeg.org/), dan [SQLite](https://www.sqlite.org/). Informasi dependensi dan lisensi tersedia di [THIRD_PARTY_NOTICES.txt](./frontend/public/THIRD_PARTY_NOTICES.txt).

## Kolaborasi

- Saat ini, proyek belum menerima pull request. Laporan masalah dan saran dapat disampaikan melalui [GitHub Issues](https://github.com/arnoldhao/xiadown/issues).
- Repositori ini dilisensikan di bawah `Apache-2.0`. Lihat [LICENSE](./LICENSE).

## Kontak

- Situs web: <https://xiadown.app/>
- Dokumentasi: <https://xiadown.app/docs/>
- Email: <xunruhao@gmail.com>
