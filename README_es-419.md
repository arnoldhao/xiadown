<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ícono de XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Una aplicación para gestionar una biblioteca multimedia y descargar videos.</strong></p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Última versión" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Licencia" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Plataformas compatibles" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Tecnologías" />
  </p>
  <p>
    <a href="https://xiadown.app/">Sitio web</a> ·
    <a href="https://xiadown.app/docs/">Documentación</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Versiones</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Reportar un problema</a> ·
    <a href="https://ko-fi.com/arnoldhao">Apoyar</a>
  </p>
  <p>
    <a href="./README.md">简体中文</a> ·
    <a href="./README_zh-Hant.md">繁體中文</a> ·
    <a href="./README_en.md">English</a> ·
    <a href="./README_ja-JP.md">日本語</a> ·
    <a href="./README_ko-KR.md">한국어</a> ·
    <strong>Español (LatAm)</strong> ·
    <a href="./README_pt-BR.md">Português (BR)</a> ·
    <a href="./README_id-ID.md">Bahasa Indonesia</a> ·
    <a href="./README_vi-VN.md">Tiếng Việt</a>
  </p>
</div>

<p align="center">
  <a href="https://xiadown.app/docs/library/">
    <img src="./images/library.webp" alt="Biblioteca de XiaDown" width="92%" />
  </a>
  <br />
  <strong>Biblioteca</strong>
</p>

## Descripción del proyecto

XiaDown es una aplicación para gestionar una biblioteca multimedia con prioridad al almacenamiento local. Admite descargas de video y detección de medios. También funciona como cliente de YouTube, YouTube Music y RSS, y permite descargar con un clic el contenido multimedia que necesites mientras navegas.

## Funciones principales

- 🗂️ **[Biblioteca](https://xiadown.app/docs/library/)** — Descarga y conversión de videos; gestión de la biblioteca.
- 🔎 **[Detección](https://xiadown.app/docs/sniff/)** — Detección y descarga de contenido multimedia en páginas web.
- 🎵 **[Música](https://xiadown.app/docs/music/)** — Explora y descarga contenido de YouTube Music; reproduce música local.
- ▶️ **[YouTube](https://xiadown.app/docs/youtube/)** — Explora, reproduce y descarga videos.
- 📡 **[RSS](https://xiadown.app/docs/rss/)** — Suscríbete a fuentes, lee artículos y descarga contenido multimedia.

## Aplicación móvil

📱 Los clientes para iPhone y iPad están en desarrollo.

## Interfaz del producto

<table>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/sniff/">
        <img src="./images/sniff-desk.webp" alt="Detección de medios en XiaDown" width="100%" />
      </a>
      <br />
      <strong>Detección</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/rss/">
        <img src="./images/rss.webp" alt="Suscripciones RSS en XiaDown" width="100%" />
      </a>
      <br />
      <strong>RSS</strong>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/music/">
        <img src="./images/youtube-music.webp" alt="Música en XiaDown" width="100%" />
      </a>
      <br />
      <strong>Música</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/youtube/">
        <img src="./images/youtube-live.webp" alt="Exploración de YouTube en XiaDown" width="100%" />
      </a>
      <br />
      <strong>YouTube</strong>
    </td>
  </tr>
</table>

## Instalación

### Homebrew

En macOS, puedes instalar XiaDown con Homebrew Cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Descargar instaladores

| Plataforma | Arquitectura | Formato | Descarga |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Instalador | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portátil | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

> La versión para macOS requiere macOS 14 (Sonoma) o posterior; la versión para Windows requiere Windows 10 o posterior.

En el primer inicio, XiaDown te guiará para configurar el idioma, la apariencia, la red y las dependencias de ejecución. Consulta [Instalación y primer inicio](https://xiadown.app/docs/start/install/) para ver los pasos detallados.

## Desarrollo local

El entorno de desarrollo requiere Go 1.25.12, Node.js 24, Bun 1.3.5 y Wails 3 alpha2.117:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
wails3 task dev
```

Consulta [Taskfile.yml](./Taskfile.yml) para conocer las demás tareas de compilación y verificación.

## Aviso legal

- XiaDown debe usarse únicamente para gestionar contenido multimedia y guardar contenido al que tengas derecho de acceso y uso.
- Cada usuario debe comprobar que la descarga, el almacenamiento, la conversión y el uso del contenido cumplan las leyes locales, las autorizaciones de los titulares de derechos y los términos de servicio de la plataforma de origen.
- No uses XiaDown para procesar contenido infractor, no autorizado, sujeto a pago o restricciones, que vulnere la privacidad o que sea ilegal.
- El usuario asume toda responsabilidad derivada del uso de XiaDown, incluidas las relacionadas con derechos de autor, normas de las plataformas, cuentas, redes y cualquier otra cuestión.

## Agradecimientos

XiaDown utiliza proyectos de código abierto como [Go](https://go.dev/), [Wails](https://v3alpha.wails.io/), [React](https://react.dev/), [yt-dlp](https://github.com/yt-dlp/yt-dlp), [FFmpeg](https://ffmpeg.org/) y [SQLite](https://www.sqlite.org/). Consulta [THIRD_PARTY_NOTICES.txt](./frontend/public/THIRD_PARTY_NOTICES.txt) para ver las dependencias y sus licencias.

## Colaboración

- Por ahora, el proyecto no acepta pull requests. Puedes enviar problemas y sugerencias mediante [GitHub Issues](https://github.com/arnoldhao/xiadown/issues).
- Este repositorio utiliza la licencia `Apache-2.0`. Consulta [LICENSE](./LICENSE).

## Contacto

- Sitio web: <https://xiadown.app/>
- Documentación: <https://xiadown.app/docs/>
- Correo electrónico: <xunruhao@gmail.com>
