<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ikon XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Alat pengunduh video bermesin ganda dengan dukungan musik online.</strong></p>
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

XiaDown adalah pemutar musik online sekaligus alat pengunduh video bermesin ganda.

Aplikasi ini dibuat untuk kreator konten: saat Anda membutuhkan materi, XiaDown menyediakan kemampuan unduh yang kuat melalui sniffing browser dan YT-DLP; saat Anda bekerja, musik online dapat tetap diputar di latar belakang. Dengan pustaka, transcoding, pengelolaan dependensi otomatis, isolasi akun, pet, dan tampilan yang dapat disesuaikan, XiaDown bisa menangani materi media sekaligus menjadi alat media desktop untuk penggunaan harian.

## Kemampuan Utama

- 📥 **Unduhan video bermesin ganda**: XiaDown menyertakan mesin unduh YT-DLP dan mesin sniffing browser berbasis CDP. Tautan biasa dapat dianalisis dan diunduh langsung, termasuk menggunakan Cookies yang tersimpan di profil koneksi; untuk halaman yang dimuat dinamis, struktur situs yang kompleks, atau resource yang membutuhkan sesi browser nyata, mode sniffing dapat menangkap video, audio, subtitle, sampul, dan resource media lainnya, lalu melanjutkannya ke transcoding dan pengelolaan pustaka.
- 🎧 **Pemutar musik desktop**: mengelola audio lokal dari resource yang diunduh secara otomatis, mendukung pencarian lagu, artis, dan playlist di YouTube Music, serta mendukung pemutaran radio YouTube Live dan penayangan video live. Pemutar menyediakan antrean, sampul, lirik tersinkron, lirik romanisasi/pinyin Asia Timur, equalizer, dan visualisasi spektrum.
- 🧩 **Ruang kerja yang bebas dan terkendali**: alat dependensi dipasang dan diperbarui otomatis tanpa mengotori lingkungan sistem; akun, Cookies, dan Profile browser dikelola melalui pengaturan koneksi yang terisolasi; tema, mode terang/gelap, warna aksen, font, ukuran font, dan Codex Pets dapat disesuaikan secara bebas.

## Kemampuan Inti

- **Unduhan sniffing**: kemampuan sniffing browser berbasis CDP yang dikembangkan sendiri, mampu mengamati video, audio, live stream, manifest, gambar, subtitle, respons API, dan resource lain di halaman. Dalam lingkungan browser nyata setelah pengguna login secara eksplisit, XiaDown dapat mengenali dan mengunduh resource dari TikTok, Douyin, Kuaishou, Xiaohongshu, dan situs sejenis, serta menghubungkan hasil unduhan langsung ke proses transcoding.
- **Unduhan YT-DLP**: mengintegrasikan YT-DLP untuk mengunduh materi dari berbagai situs video online, dengan dukungan stabil untuk platform umum seperti YouTube dan Bilibili. Tempel tautan untuk menganalisis dan menyimpan video, audio, subtitle, dan sampul; unduhan juga dapat menggunakan Cookies yang tersimpan di profil koneksi untuk konten yang memang berhak diakses pengguna, lalu dilanjutkan ke transcoding dan pengelolaan pustaka.
- **Transcoding audio dan video**: berbasis FFmpeg, mendukung transcoding setelah unduhan maupun pemilihan file lokal secara manual. Preset bawaan mencakup H.264, H.265, VP9, MP3, AAC, Opus, FLAC, WAV, serta target output umum seperti ukuran asli, 2160p, 1080p, 720p, dan 480p.
- **Manajemen resource multi-tampilan**: tampilan tugas dan tampilan file menyatukan unduhan, hasil transcoding, subtitle, sampul, dan file impor. Mendukung pratinjau media, detail tugas, detail file, pemilihan massal, penghapusan, pemulihan tugas gagal, pemeriksaan keberadaan file, dan pembersihan catatan yang sudah tidak valid.
- **Pemutaran musik lokal**: mengindeks file audio di pustaka secara otomatis, dengan pemutaran lokal, antrean pemutaran, tampilan sampul, lirik tersinkron, lirik romanisasi/pinyin Asia Timur, equalizer, dan beberapa gaya visualisasi spektrum.
- **YouTube Music**: menghadirkan pengalaman YouTube Music di desktop, dengan koneksi akun, pencarian lagu/artis/playlist, rekomendasi beranda, pustaka playlist, artis yang diikuti, musik yang disukai, antrean pemutaran, lirik, serta pembersihan data iklan untuk mengurangi gangguan pemutaran.
- **YouTube Live**: mendukung penambahan grup dan channel YouTube Live kustom, melihat status live, memutar radio live, dan menonton video live secara langsung.
- **Manajemen dependensi otomatis**: secara otomatis memelihara instalasi, verifikasi, dan pembaruan YT-DLP, FFmpeg, Bun, dan alat terkait. Path alat dikelola secara independen oleh aplikasi, sehingga tidak bergantung pada dan tidak mengotori lingkungan global pengguna.
- **Isolasi kredensial dan pengguna**: mendukung pemanggilan kemampuan browser lokal melalui CDP sekaligus menyimpan Profiles dan Cookies yang independen. Data hanya berasal dari login yang dilakukan pengguna, dan pengaturan koneksi tetap terpisah dari penggunaan browser sehari-hari.
- **Kustomisasi tampilan**: mendukung paket tema, mode terang/gelap/otomatis, warna aksen, font, ukuran font, gaya sidebar, dan lainnya. Codex Pets Gallery bawaan dapat mengimpor Pets online maupun lokal sebagai elemen pendamping di desktop.

## Pratinjau Produk

<p align="center">
  <img src="./images/download.webp" alt="Tampilan tugas unduhan XiaDown" width="88%" />
  <br />
  <sub>Tugas unduhan dan transcoding</sub>
</p>

<p align="center">
  <img src="./images/sniff-desk.webp" alt="Tampilan sniffing resource XiaDown" width="88%" />
  <br />
  <sub>Sniffing desk untuk menangkap resource halaman web</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="Tampilan pemutaran YouTube Music di XiaDown" width="88%" />
  <br />
  <sub>Pemutaran YouTube Music di desktop</sub>
</p>

<p align="center">
  <img src="./images/youtube-live.webp" alt="Tampilan video YouTube Live di XiaDown" width="88%" />
  <br />
  <sub>Penayangan video YouTube Live</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="Tampilan pustaka XiaDown" width="88%" />
  <br />
  <sub>Pustaka terpadu untuk unduhan dan konten hasil transcoding</sub>
</p>

<details>
  <summary>Tampilan pengaturan dan personalisasi lainnya</summary>

  <p align="center">
    <img src="./images/connector.webp" alt="Tampilan koneksi dan isolasi akun XiaDown" width="88%" />
    <br />
    <sub>Isolasi pengaturan koneksi, Cookies, dan Profile browser</sub>
  </p>

  <p align="center">
    <img src="./images/tools.webp" alt="Tampilan manajemen alat dependensi XiaDown" width="88%" />
    <br />
    <sub>Manajemen otomatis YT-DLP, FFmpeg, dan Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="Tampilan pengaturan visual XiaDown" width="88%" />
    <br />
    <sub>Tema, mode terang/gelap, warna aksen, font, dan ukuran font</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="Tampilan Codex Pets Gallery di XiaDown" width="88%" />
    <br />
    <sub>Codex Pets Gallery dan impor Pet lokal</sub>
  </p>
</details>

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
