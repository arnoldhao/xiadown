<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Icono de XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Una herramienta de descarga de video con doble motor y soporte de música en línea.</strong></p>
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

XiaDown es un reproductor de música en línea y también una herramienta de descarga de video con doble motor.

Está pensado para creadores de contenido: cuando necesitas material, ofrece descargas potentes mediante captura de recursos del navegador y YT-DLP; cuando necesitas trabajar, mantiene música en línea reproduciéndose en segundo plano. Con biblioteca, transcodificación, gestión automática de dependencias, aislamiento de cuentas, mascotas y personalización visual, XiaDown sirve tanto para manejar material multimedia como para convertirse en tu herramienta multimedia de escritorio del día a día.

## Capacidades Principales

- 📥 **Descarga de video con doble motor**: incluye un motor de descarga YT-DLP y un motor de captura del navegador basado en CDP. Los enlaces normales se pueden analizar y descargar directamente, incluso usando Cookies guardadas en los perfiles de conexión; para páginas con carga dinámica, estructuras de sitio complejas o recursos que requieren una sesión real del navegador, el modo de captura puede detectar video, audio, subtítulos, portadas y otros recursos multimedia, y luego continuar con transcodificación y gestión en la biblioteca.
- 🎧 **Reproductor de música de escritorio**: administra automáticamente el audio local de los recursos descargados, permite buscar canciones, artistas y playlists en YouTube Music, y reproduce radios de YouTube Live con visualización de video en vivo. El reproductor incluye cola, carátulas, letras sincronizadas, letras romanizadas/pinyin de Asia oriental, ecualizador y visualizaciones de espectro.
- 🧩 **Un espacio flexible y bajo control**: las herramientas dependientes se instalan y actualizan automáticamente sin contaminar el entorno del sistema; las cuentas, Cookies y Profiles del navegador se gestionan mediante conexiones aisladas; temas, modo claro/oscuro, colores de acento, fuentes, tamaños de fuente y Codex Pets se pueden ajustar libremente.

## Capacidades Clave

- **Descarga por captura de recursos**: una capacidad propia de captura del navegador basada en CDP que puede observar video, audio, transmisiones en vivo, manifiestos, imágenes, subtítulos, respuestas de API y otros recursos de una página. En un entorno real de navegador donde el usuario inició sesión de forma explícita, puede identificar y descargar recursos de TikTok, Douyin, Kuaishou, Xiaohongshu y sitios similares, además de enlazar la descarga directamente con la transcodificación.
- **Descargas con YT-DLP**: integra YT-DLP para descargar material desde una amplia variedad de sitios de video en línea, con soporte estable para plataformas comunes como YouTube y Bilibili. Pega un enlace para analizar y guardar video, audio, subtítulos y portadas; también puede usar Cookies guardadas en perfiles de conexión para contenido al que el usuario está autorizado a acceder, y luego continuar con transcodificación y gestión en la biblioteca.
- **Transcodificación de audio y video**: impulsada por FFmpeg, admite transcodificación después de la descarga y selección manual de archivos locales. Incluye presets para H.264, H.265, VP9, MP3, AAC, Opus, FLAC, WAV y salidas comunes como tamaño original, 2160p, 1080p, 720p y 480p.
- **Gestión de recursos con varias vistas**: las vistas de tareas y archivos unifican descargas, transcodificaciones, subtítulos, portadas y archivos importados. Incluye vista previa de medios, detalles de tareas, detalles de archivos, selección por lotes, eliminación, recuperación de tareas fallidas, verificación de existencia de archivos y limpieza de registros obsoletos.
- **Reproducción de música local**: indexa automáticamente archivos de audio en la biblioteca, con reproducción local, cola, carátulas, letras sincronizadas, letras romanizadas/pinyin de Asia oriental, ecualizador y varios estilos de visualización de espectro.
- **YouTube Music**: ofrece una experiencia de YouTube Music en escritorio con conexiones de cuenta, búsqueda de canciones/artistas/playlists, recomendaciones de inicio, biblioteca de playlists, artistas seguidos, música marcada como favorita, cola de reproducción, letras y limpieza de datos publicitarios para reducir interrupciones.
- **YouTube Live**: permite crear grupos y canales personalizados de YouTube Live, ver el estado de transmisiones, reproducir radios en vivo y ver directamente el video en vivo.
- **Gestión automática de dependencias**: mantiene automáticamente la instalación, verificación y actualización de YT-DLP, FFmpeg, Bun y herramientas relacionadas. Las rutas de herramientas son gestionadas por la app de forma independiente, sin depender del entorno global del usuario ni contaminarlo.
- **Aislamiento de credenciales y usuario**: permite usar capacidades del navegador local mediante CDP y persistir Profiles y Cookies independientes. Los datos provienen solo de inicios de sesión iniciados por el usuario, y la configuración de conexiones queda separada del uso cotidiano del navegador.
- **Personalización visual**: admite paquetes de temas, modos claro/oscuro/automático, colores de acento, fuentes, tamaños de fuente, estilos de barra lateral y más. Codex Pets Gallery integrado permite importar Pets en línea o locales y usarlos como elementos de compañía en el escritorio.

## Vista del Producto

<p align="center">
  <img src="./images/download.webp" alt="Vista de tareas de descarga de XiaDown" width="88%" />
  <br />
  <sub>Tareas de descarga y transcodificación</sub>
</p>

<p align="center">
  <img src="./images/sniff-desk.webp" alt="Vista de captura de recursos de XiaDown" width="88%" />
  <br />
  <sub>Panel de captura para recursos de páginas web</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="Vista de reproducción de YouTube Music en XiaDown" width="88%" />
  <br />
  <sub>Reproducción de YouTube Music en escritorio</sub>
</p>

<p align="center">
  <img src="./images/youtube-live.webp" alt="Vista de video de YouTube Live en XiaDown" width="88%" />
  <br />
  <sub>Visualización de video en YouTube Live</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="Vista de biblioteca de XiaDown" width="88%" />
  <br />
  <sub>Biblioteca unificada para descargas y contenido transcodificado</sub>
</p>

<details>
  <summary>Más vistas de configuración y personalización</summary>

  <p align="center">
    <img src="./images/connector.webp" alt="Vista de conexiones y aislamiento de cuentas de XiaDown" width="88%" />
    <br />
    <sub>Aislamiento de conexiones, Cookies y Profiles del navegador</sub>
  </p>

  <p align="center">
    <img src="./images/tools.webp" alt="Vista de gestión de herramientas dependientes de XiaDown" width="88%" />
    <br />
    <sub>Gestión automática de YT-DLP, FFmpeg y Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="Vista de configuración visual de XiaDown" width="88%" />
    <br />
    <sub>Temas, modo claro/oscuro, colores de acento, fuentes y tamaños</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="Vista de Codex Pets Gallery en XiaDown" width="88%" />
    <br />
    <sub>Codex Pets Gallery e importación de Pets locales</sub>
  </p>
</details>

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

## Aviso Legal

- XiaDown se ofrece como una herramienta auxiliar para gestión de medios y descargas, destinada al aprendizaje, la investigación y el guardado de contenido al que tengas autorización para acceder y usar.
- Eres responsable de confirmar que cualquier descarga, almacenamiento, conversión o uso de contenido cuenta con autorización del titular de derechos y cumple con las leyes aplicables y los términos del sitio/plataforma de destino.
- No uses XiaDown para descargar, distribuir, vender o explotar de cualquier otro modo contenido infractor, no autorizado, de pago/restringido, privado o ilegal.
- Cualquier responsabilidad legal relacionada con derechos de autor, reglas de plataforma, cuentas, red u otros asuntos derivada del uso de XiaDown corresponde al usuario; los mantenedores del proyecto no son responsables por la conducta de los usuarios ni por sus consecuencias.

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
