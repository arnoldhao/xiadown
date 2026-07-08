<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Icono de XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Una herramienta de descarga de video con doble motor y soporte de música en línea.</strong></p>
  <p>Listen Keep, Make it Yours</p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Última versión" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Licencia" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Plataformas compatibles" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Stack tecnológico" />
  </p>
  <p>
    <a href="https://xiadown.app/">Sitio web</a> ·
    <a href="https://xiadown.app/es-419/docs/">Guía de uso</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Versiones</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Issues</a> ·
    <a href="https://ko-fi.com/arnoldhao">Patrocinar</a>
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
  <img src="./images/download.webp" alt="Vista de tareas de descarga de XiaDown" width="92%" />
  <br />
  <sub>Tareas de descarga y transcodificación</sub>
</p>

## Descripción General

XiaDown es un reproductor de música en línea y también una herramienta de descarga de video con doble motor.

Es una herramienta diaria creada para creadores de contenido: cuando necesitas material, descargas con captura de recursos y YT-DLP; cuando necesitas concentrarte, mantienes música en línea reproduciéndose en segundo plano; sus opciones de personalización ayudan a que el uso a largo plazo siga siendo cómodo y fresco.

## Capacidades Principales

### 📥 Descarga y Transcodificación

- **Descarga por captura de recursos**: observa video, audio, transmisiones en vivo, manifiestos, imágenes, subtítulos y respuestas de API mediante CDP; es útil para sitios como TikTok, Douyin, Kuaishou y Xiaohongshu que requieren una sesión real del navegador.
- **Descargas con YT-DLP**: pega un enlace para analizar plataformas comunes como YouTube y Bilibili, guardar video, audio, subtítulos y portadas, y usar una identidad con sesión iniciada para descargar contenido al que tienes autorización de acceso.
- **Transcodificación de audio y video**: basada en FFmpeg, admite transcodificación después de la descarga y transcodificación de archivos locales, con presets integrados como H.264, H.265, VP9, MP3, AAC, Opus, FLAC y WAV.

### 🗂️ Gestión de Recursos

- **Gestión de recursos con varias vistas**: las vistas de tareas y archivos unifican descargas, transcodificaciones, subtítulos, portadas y archivos importados, con vista previa, detalles, selección por lotes, eliminación, recuperación de fallos y limpieza de registros obsoletos.

### 🎧 Reproductor

- **Reproducción de música local**: indexa automáticamente el audio de la biblioteca y admite cola, carátulas, letras sincronizadas, letras romanizadas/pinyin de Asia oriental, ecualizador y visualizaciones de espectro.
- **YouTube Music**: busca canciones, artistas y playlists con una experiencia de escritorio, con recomendaciones de inicio, biblioteca de playlists, artistas seguidos, música marcada como favorita, cola de reproducción y letras.
- **YouTube Live**: crea grupos y canales en vivo personalizados, consulta el estado de transmisiones, reproduce radio en vivo y abre directamente el video en vivo.

### 🔐 Seguridad y Aislamiento

- **Gestión automática de dependencias**: instala, verifica y actualiza YT-DLP, FFmpeg, Bun y herramientas relacionadas automáticamente; las rutas son mantenidas por la app y no contaminan el entorno del sistema.
- **Aislamiento de credenciales y usuario**: los datos de sesiones de app provienen de inicios de sesión iniciados por el usuario y se almacenan de forma independiente con cifrado del sistema en macOS y Windows; las conexiones quedan separadas del navegador de uso diario.

### 🎨 Libertad

- **Personalización visual**: admite paquetes de temas, modos claro/oscuro/automático, colores de acento, fuentes, tamaños de fuente y estilos de barra lateral; Codex Pets Gallery integrado puede importar Pets en línea y locales.

## Vista del Producto

<p align="center">
  <img src="./images/sniff-desk.webp" alt="Vista de captura de recursos de XiaDown" width="92%" />
  <br />
  <sub>Panel de captura para recursos de páginas web</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="Vista de reproducción de YouTube Music en XiaDown" width="92%" />
  <br />
  <sub>Reproducción de YouTube Music en escritorio</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="Vista de biblioteca de XiaDown" width="92%" />
  <br />
  <sub>Biblioteca unificada para descargas y contenido transcodificado</sub>
</p>

<details>
  <summary><strong>Más capturas de la interfaz</strong></summary>

  <p align="center">
    <img src="./images/youtube-live.webp" alt="Vista de video de YouTube Live en XiaDown" width="92%" />
    <br />
    <sub>Visualización de video en YouTube Live</sub>
  </p>

  <p align="center">
    <img src="./images/app_sessions.webp" alt="Vista de sesiones de app y estado de inicio de sesión de XiaDown" width="92%" />
    <br />
    <sub>Sesiones de app, verificación de inicio de sesión y estado de cuentas</sub>
  </p>

  <p align="center">
    <img src="./images/settings_download.webp" alt="Vista de ajustes de descarga y gestión de herramientas dependientes de XiaDown" width="92%" />
    <br />
    <sub>Directorio de descarga, concurrencia y gestión de YT-DLP, FFmpeg y Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="Vista de configuración visual de XiaDown" width="92%" />
    <br />
    <sub>Temas, modo claro/oscuro, colores de acento, fuentes y tamaños</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="Vista de Codex Pets Gallery en XiaDown" width="92%" />
    <br />
    <sub>Codex Pets Gallery e importación de Pets locales</sub>
  </p>
</details>

## Instalación

### Homebrew

En macOS, instala con Homebrew cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Descargar instaladores

Descarga directamente el paquete más reciente a continuación. Las versiones anteriores están disponibles en [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Plataforma | Arquitectura | Paquete | Descargar |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Instalador | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portable | [Descargar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### Primer inicio

1. `macOS`: abre el `.dmg`, arrastra `XiaDown.app` a la carpeta Aplicaciones y ábrela.
2. `Windows`: ejecuta directamente el instalador `.exe`, o descomprime el paquete portable y ábrelo. Si SmartScreen aparece en el primer inicio, elige `Más información -> Ejecutar de todas formas`.
3. XiaDown abre un flujo de bienvenida para configurar idioma, tema, proxy y dependencias. Los flujos principales están en la bienvenida y en la interfaz.

### Compatibilidad de Navegadores CDP

Navegadores compatibles actualmente:

| Principales | Privacidad y eficiencia | Especializados y regionales |
| --- | --- | --- |
| Chrome, Chromium, Edge | Brave, Vivaldi, Arc, Helium | Opera, Opera GX, Yandex Browser |

## Desarrollo local

Después de preparar Go y Bun, instala la CLI de Wails 3 e inicia el modo de desarrollo:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.95
wails3 dev
```

## Aviso Legal

- XiaDown se ofrece como una herramienta auxiliar para gestión de medios y descargas, destinada al aprendizaje, la investigación y el guardado de contenido al que tengas autorización para acceder y usar.
- Eres responsable de confirmar que cualquier descarga, almacenamiento, conversión o uso de contenido cuenta con autorización del titular de derechos y cumple con las leyes aplicables y los términos del sitio/plataforma de destino.
- No uses XiaDown para descargar, distribuir, vender o explotar de cualquier otro modo contenido infractor, no autorizado, de pago/restringido, privado o ilegal.
- Cualquier responsabilidad legal relacionada con derechos de autor, reglas de plataforma, cuentas, red u otros asuntos derivada del uso de XiaDown corresponde al usuario; los mantenedores del proyecto no son responsables por la conducta de los usuarios ni por sus consecuencias.

## Agradecimientos

XiaDown se construye sobre excelentes proyectos de código abierto. La experiencia de escritorio, el procesamiento multimedia, el almacenamiento local, las conexiones con navegadores, la música en línea y las capacidades de interfaz dependen de estas bases.

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

- Sitio web: <https://xiadown.app/>
- Guía de uso: <https://xiadown.app/es-419/docs/>
- Correo: <xunruhao@gmail.com>
