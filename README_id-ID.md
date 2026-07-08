<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ikon XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Alat pengunduh video bermesin ganda dengan dukungan musik online.</strong></p>
  <p>Listen Keep, Make it Yours</p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Versi terbaru" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Lisensi" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Platform yang didukung" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Stack teknologi" />
  </p>
  <p>
    <a href="https://xiadown.app/">Situs web</a> ·
    <a href="https://xiadown.app/id-id/docs/">Panduan Penggunaan</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Rilis</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Issues</a> ·
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
  <img src="./images/download.webp" alt="Tampilan tugas unduhan XiaDown" width="92%" />
  <br />
  <sub>Tugas unduhan dan transcoding</sub>
</p>

## Gambaran Umum

XiaDown adalah pemutar musik online sekaligus alat pengunduh video bermesin ganda.

XiaDown dibuat sebagai alat harian untuk kreator konten: saat membutuhkan materi, gunakan sniffing dan YT-DLP untuk mengunduh; saat perlu fokus, putar musik online di latar belakang; opsi kustomisasi yang kaya membuat penggunaan jangka panjang tetap ringan dan terasa segar.

## Kemampuan Utama

### 📥 Unduh dan Transcoding

- **Unduhan sniffing**: mengamati video, audio, live stream, manifest, gambar, subtitle, dan respons API lewat CDP; cocok untuk situs seperti TikTok, Douyin, Kuaishou, dan Xiaohongshu yang membutuhkan sesi browser nyata.
- **Unduhan YT-DLP**: tempel tautan untuk menganalisis platform umum seperti YouTube dan Bilibili, menyimpan video, audio, subtitle, dan sampul, serta menggunakan identitas yang sudah login untuk mengunduh konten yang berhak Anda akses.
- **Transcoding audio dan video**: berbasis FFmpeg, mendukung transcoding setelah unduhan dan transcoding file lokal, dengan preset bawaan seperti H.264, H.265, VP9, MP3, AAC, Opus, FLAC, dan WAV.

### 🗂️ Manajemen Resource

- **Manajemen resource multi-tampilan**: tampilan tugas dan tampilan file menyatukan unduhan, hasil transcoding, subtitle, sampul, dan file impor, lengkap dengan pratinjau, detail, pemilihan massal, penghapusan, pemulihan kegagalan, dan pembersihan catatan usang.

### 🎧 Pemutar

- **Pemutaran musik lokal**: mengindeks audio pustaka secara otomatis dan mendukung antrean, sampul, lirik tersinkron, lirik romanisasi/pinyin Asia Timur, equalizer, dan visualisasi spektrum.
- **YouTube Music**: cari lagu, artis, dan playlist dalam pengalaman desktop, dengan rekomendasi beranda, pustaka playlist, artis yang diikuti, musik yang disukai, antrean pemutaran, dan lirik.
- **YouTube Live**: buat grup dan channel live kustom, lihat status live, putar radio live, dan buka video live secara langsung.

### 🔐 Keamanan dan Isolasi

- **Manajemen dependensi otomatis**: memasang, memverifikasi, dan memperbarui YT-DLP, FFmpeg, Bun, dan alat terkait secara otomatis; path alat dikelola aplikasi dan tidak mengotori lingkungan sistem.
- **Isolasi kredensial dan pengguna**: data sesi aplikasi berasal dari login yang dimulai pengguna dan disimpan terpisah dengan enkripsi sistem di macOS dan Windows; pengaturan koneksi tetap terpisah dari penggunaan browser sehari-hari.

### 🎨 Kebebasan

- **Kustomisasi tampilan**: mendukung paket tema, mode terang/gelap/otomatis, warna aksen, font, ukuran font, dan gaya sidebar; Codex Pets Gallery bawaan dapat mengimpor Pets online maupun lokal.

## Pratinjau Produk

<p align="center">
  <img src="./images/sniff-desk.webp" alt="Tampilan sniffing resource XiaDown" width="92%" />
  <br />
  <sub>Sniffing desk untuk menangkap resource halaman web</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="Tampilan pemutaran YouTube Music di XiaDown" width="92%" />
  <br />
  <sub>Pemutaran YouTube Music di desktop</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="Tampilan pustaka XiaDown" width="92%" />
  <br />
  <sub>Pustaka terpadu untuk unduhan dan konten hasil transcoding</sub>
</p>

<details>
  <summary><strong>Cuplikan antarmuka lainnya</strong></summary>

  <p align="center">
    <img src="./images/youtube-live.webp" alt="Tampilan video YouTube Live di XiaDown" width="92%" />
    <br />
    <sub>Penayangan video YouTube Live</sub>
  </p>

  <p align="center">
    <img src="./images/app_sessions.webp" alt="Tampilan sesi aplikasi dan status masuk XiaDown" width="92%" />
    <br />
    <sub>Sesi aplikasi, verifikasi masuk, dan manajemen status akun</sub>
  </p>

  <p align="center">
    <img src="./images/settings_download.webp" alt="Tampilan pengaturan unduhan dan manajemen alat dependensi XiaDown" width="92%" />
    <br />
    <sub>Direktori unduhan, konkurensi, dan manajemen YT-DLP, FFmpeg, Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="Tampilan pengaturan visual XiaDown" width="92%" />
    <br />
    <sub>Tema, mode terang/gelap, warna aksen, font, dan ukuran font</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="Tampilan Codex Pets Gallery di XiaDown" width="92%" />
    <br />
    <sub>Codex Pets Gallery dan impor Pet lokal</sub>
  </p>
</details>

## Cara Instalasi

### Homebrew

Di macOS, instal dengan Homebrew cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Unduh installer

Unduh paket terbaru secara langsung di bawah ini. Versi lama tersedia di [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Platform | Arsitektur | Paket | Unduh |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Installer | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portabel | [Unduh](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### Peluncuran pertama

1. `macOS`: buka `.dmg`, tarik `XiaDown.app` ke folder Applications, lalu buka aplikasinya.
2. `Windows`: jalankan installer `.exe` secara langsung, atau ekstrak paket portabel dan buka aplikasinya. Jika SmartScreen muncul saat pertama kali dibuka, pilih `More info -> Run anyway`.
3. XiaDown membuka alur onboarding untuk pengaturan bahasa, tema, proxy, dan dependensi. Alur utama berada di onboarding dan UI aplikasi.

### Dukungan Browser CDP

Browser yang saat ini didukung:

| Utama | Privasi dan efisiensi | Khusus dan regional |
| --- | --- | --- |
| Chrome, Chromium, Edge | Brave, Vivaldi, Arc, Helium | Opera, Opera GX, Yandex Browser |

## Pengembangan lokal

Setelah menyiapkan Go dan Bun, instal Wails 3 CLI lalu jalankan mode pengembangan:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.95
wails3 dev
```

## Penafian

- XiaDown disediakan sebagai alat bantu manajemen media dan pengunduhan, untuk pembelajaran, riset, dan penyimpanan konten yang memang berhak Anda akses dan gunakan.
- Anda bertanggung jawab untuk memastikan setiap pengunduhan, penyimpanan, konversi, atau penggunaan konten telah diizinkan oleh pemegang hak dan mematuhi hukum yang berlaku serta ketentuan situs/platform tujuan.
- Jangan gunakan XiaDown untuk mengunduh, mendistribusikan, menjual, atau mengeksploitasi konten yang melanggar hak, tidak berizin, berbayar/terbatas, bersifat privat, atau melanggar hukum.
- Setiap tanggung jawab hukum terkait hak cipta, kebijakan platform, akun, jaringan, atau hal lain yang timbul dari penggunaan XiaDown menjadi tanggung jawab pengguna; pengelola proyek tidak bertanggung jawab atas tindakan pengguna maupun konsekuensinya.

## Ucapan Terima Kasih

XiaDown dibangun di atas proyek open source yang sangat baik. Pengalaman desktop, pemrosesan media, penyimpanan lokal, koneksi browser, musik online, dan kemampuan antarmuka semuanya bergantung pada fondasi ini.

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

- Situs web: <https://xiadown.app/>
- Panduan Penggunaan: <https://xiadown.app/id-id/docs/>
- Email: <xunruhao@gmail.com>
