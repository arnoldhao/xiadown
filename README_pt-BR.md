<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ícone do XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Um aplicativo para gerenciar uma biblioteca de mídia e baixar vídeos.</strong></p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Versão mais recente" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Licença" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Plataformas compatíveis" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Tecnologias" />
  </p>
  <p>
    <a href="https://xiadown.app/">Site</a> ·
    <a href="https://xiadown.app/docs/">Documentação</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Versões</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Relatar um problema</a> ·
    <a href="https://ko-fi.com/arnoldhao">Apoiar</a>
  </p>
  <p>
    <a href="./README.md">简体中文</a> ·
    <a href="./README_zh-Hant.md">繁體中文</a> ·
    <a href="./README_en.md">English</a> ·
    <a href="./README_ja-JP.md">日本語</a> ·
    <a href="./README_ko-KR.md">한국어</a> ·
    <a href="./README_es-419.md">Español (LatAm)</a> ·
    <strong>Português (BR)</strong> ·
    <a href="./README_id-ID.md">Bahasa Indonesia</a> ·
    <a href="./README_vi-VN.md">Tiếng Việt</a>
  </p>
</div>

<p align="center">
  <a href="https://xiadown.app/docs/library/">
    <img src="./images/library.webp" alt="Biblioteca do XiaDown" width="92%" />
  </a>
  <br />
  <strong>Biblioteca</strong>
</p>

## Sobre o projeto

O XiaDown é um aplicativo para gerenciar uma biblioteca de mídia com prioridade ao armazenamento local. Oferece download de vídeos e captura de mídia, além de funcionar como cliente para YouTube, YouTube Music e RSS. Você pode baixar com um clique a mídia necessária enquanto navega.

## Principais recursos

- 🗂️ **[Biblioteca](https://xiadown.app/docs/library/)** — Download e conversão de vídeos; gerenciamento da biblioteca.
- 🔎 **[Captura](https://xiadown.app/docs/sniff/)** — Captura e download de mídia em páginas da web.
- 🎵 **[Música](https://xiadown.app/docs/music/)** — Navegação e download de conteúdo no YouTube Music; reprodução de músicas locais.
- ▶️ **[YouTube](https://xiadown.app/docs/youtube/)** — Navegação, reprodução e download de vídeos.
- 📡 **[RSS](https://xiadown.app/docs/rss/)** — Assinatura de feeds, leitura de conteúdo e download de mídia.

## Aplicativo móvel

📱 Os aplicativos para iPhone e iPad estão em desenvolvimento.

## Interface do produto

<table>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/sniff/">
        <img src="./images/sniff-desk.webp" alt="Captura de mídia no XiaDown" width="100%" />
      </a>
      <br />
      <strong>Captura</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/rss/">
        <img src="./images/rss.webp" alt="Assinaturas RSS no XiaDown" width="100%" />
      </a>
      <br />
      <strong>RSS</strong>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/music/">
        <img src="./images/youtube-music.webp" alt="Música no XiaDown" width="100%" />
      </a>
      <br />
      <strong>Música</strong>
    </td>
    <td align="center" width="50%">
      <a href="https://xiadown.app/docs/youtube/">
        <img src="./images/youtube-live.webp" alt="Navegação no YouTube pelo XiaDown" width="100%" />
      </a>
      <br />
      <strong>YouTube</strong>
    </td>
  </tr>
</table>

## Instalação

### Homebrew

No macOS, você pode instalar o XiaDown com o Homebrew Cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Baixar os instaladores

| Plataforma | Arquitetura | Formato | Download |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Baixar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Baixar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Instalador | [Baixar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portátil | [Baixar](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

> A versão para macOS requer o macOS 14 (Sonoma) ou posterior; a versão para Windows requer o Windows 10 ou posterior.

Na primeira execução, o XiaDown orientará você na configuração de idioma, aparência, rede e dependências de execução. Consulte [Instalação e primeira execução](https://xiadown.app/docs/start/install/) para ver as instruções detalhadas.

## Desenvolvimento local

O ambiente de desenvolvimento requer Go 1.25.12, Node.js 24, Bun 1.3.5 e Wails 3 alpha2.117:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
wails3 task dev
```

Consulte [Taskfile.yml](./Taskfile.yml) para conhecer as demais tarefas de compilação e verificação.

## Aviso legal

- O XiaDown deve ser usado apenas para gerenciar mídia e salvar conteúdo que você tenha o direito de acessar e usar.
- Cada usuário deve verificar se o download, armazenamento, conversão e uso do conteúdo estão de acordo com as leis locais, as autorizações dos detentores dos direitos e os termos de serviço da plataforma de origem.
- Não use o XiaDown para processar conteúdo infrator, não autorizado, com acesso pago ou restrito, que viole a privacidade ou que seja ilegal.
- O usuário assume toda a responsabilidade por questões de direitos autorais, regras das plataformas, contas, redes e quaisquer outras decorrentes do uso do XiaDown.

## Agradecimentos

O XiaDown utiliza projetos de código aberto como [Go](https://go.dev/), [Wails](https://v3alpha.wails.io/), [React](https://react.dev/), [yt-dlp](https://github.com/yt-dlp/yt-dlp), [FFmpeg](https://ffmpeg.org/) e [SQLite](https://www.sqlite.org/). Consulte [THIRD_PARTY_NOTICES.txt](./frontend/public/THIRD_PARTY_NOTICES.txt) para ver as dependências e suas licenças.

## Colaboração

- No momento, o projeto não aceita pull requests. Envie problemas e sugestões pela página de [GitHub Issues](https://github.com/arnoldhao/xiadown/issues).
- Este repositório utiliza a licença `Apache-2.0`. Consulte [LICENSE](./LICENSE).

## Contato

- Site: <https://xiadown.app/>
- Documentação: <https://xiadown.app/docs/>
- E-mail: <xunruhao@gmail.com>
