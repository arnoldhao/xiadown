<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Icono de XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Una herramienta para descargar videos con soporte de música en línea.</strong></p>
  <p>Listen Keep, Make it Yours</p>
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
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Última versión" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Licencia" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Plataformas compatibles" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Stack tecnológico" />
  </p>
</div>

## Descripción General

XiaDown es un reproductor de música en línea y también una herramienta para descargar videos.

Está creado para creadores de contenido: cuando necesitas material, ofrece una potente descarga basada en YT-DLP; cuando necesitas trabajar, mantiene la música en línea reproduciéndose en segundo plano. Con mascotas y apariencia personalizable, la app se mantiene simple sin sentirse aburrida.

## Capacidades Principales

- **Reproductor de música en línea**: un reproductor de escritorio diseñado para estaciones Lo-Fi de YouTube y YouTube Music, con inicio de sesión, búsqueda de canciones, artistas y playlists, cola de reproducción, letras, carátulas y soporte para estaciones Lo-Fi en línea personalizadas. Las pistas que quieras conservar se pueden descargar a la biblioteca local.
- **Descargas de video y audio**: impulsado por YT-DLP, con soporte para descargar material desde miles de sitios de video en línea; pega un enlace para guardar video, audio, subtítulos y portadas, luego transcodifica y administra todo en la biblioteca local.
- **Espacio multimedia personalizado**: paquetes de temas cuidadosamente diseñados, colores de acento, modos de apariencia, estilos de barra lateral y soporte completo para Codex Pets, con dependencias y actualizaciones de la app mantenidas automáticamente para el uso diario a largo plazo.

## Vista del Producto

<p align="center">
  <img src="./images/download.png" alt="Vista de tareas de descarga de XiaDown" width="88%" />
</p>

<p align="center">
  <img src="./images/listen.png" alt="Vista de reproducción de música en línea Listen de XiaDown" width="88%" />
</p>

<p align="center">
  <img src="./images/library.png" alt="Vista de biblioteca de XiaDown" width="88%" />
</p>

## Inicio Rápido

### Descargar e instalar

Descarga directamente el instalador más reciente a continuación. Las versiones anteriores están disponibles en [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Plataforma | Arquitectura | Paquete | Descargar |
| --- | --- | --- | --- |
| macOS | Apple Silicon | Archivo comprimido | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | Archivo comprimido | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | Instalador | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portable | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### Primer inicio

1. `macOS`: descomprime el paquete y mueve `XiaDown.app` a la carpeta Aplicaciones. Si macOS indica que la app no se puede abrir o está dañada, ejecuta `sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app` en la terminal.
2. `Windows`: ejecuta directamente el instalador `.exe`, o descomprime el paquete portable y ábrelo. Si SmartScreen aparece en el primer inicio, elige `Más información -> Ejecutar de todas formas`.
3. XiaDown abre un flujo de bienvenida para configurar idioma, tema, proxy y dependencias. Los flujos principales están en la bienvenida y en la interfaz.

## Agradecimientos

XiaDown se construye sobre excelentes proyectos de código abierto. La experiencia de escritorio, el procesamiento multimedia, el almacenamiento local, las conexiones con navegadores, la música en línea y la interfaz frontend dependen de estas bases.

| Categoría | Página principal |
| --- | --- |
| Framework de escritorio | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| Procesamiento multimedia | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| Almacenamiento local | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| Conexiones con navegadores | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| Experiencia frontend | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## Colaboración

- El proyecto está en desarrollo activo y por ahora no acepta pull requests. Se agradecen comentarios, reportes de errores y escenarios de uso a través de [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) o por correo.
- Este repositorio tiene licencia `Apache-2.0`. Consulta [LICENSE](./LICENSE).

## Contacto

- Sitio web: <https://xiadown.dreamapp.cc/>
- Correo: <xunruhao@gmail.com>
