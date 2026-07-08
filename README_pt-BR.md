<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ícone do XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Uma ferramenta de download de vídeo com dois mecanismos e suporte a música online.</strong></p>
  <p>Listen Keep, Make it Yours</p>
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Versão mais recente" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Licença" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Plataformas compatíveis" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Stack tecnológico" />
  </p>
  <p>
    <a href="https://xiadown.app/">Site</a> ·
    <a href="https://xiadown.app/pt-br/docs/">Guia de uso</a> ·
    <a href="https://github.com/arnoldhao/xiadown/releases">Versões</a> ·
    <a href="https://github.com/arnoldhao/xiadown/issues">Issues</a> ·
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
  <img src="./images/download.webp" alt="Tela de tarefas de download do XiaDown" width="92%" />
  <br />
  <sub>Tarefas de download e transcodificação</sub>
</p>

## Visão Geral

XiaDown é um reprodutor de música online e também uma ferramenta de download de vídeo com dois mecanismos.

Ele é uma ferramenta diária criada para criadores de conteúdo: quando você precisa de material, baixa com captura de recursos e YT-DLP; quando precisa se concentrar, mantém música online tocando em segundo plano; as opções ricas de personalização ajudam o uso contínuo a seguir leve e renovado.

## Principais Recursos

### 📥 Download e Transcodificação

- **Download por captura de recursos**: observa vídeo, áudio, transmissões ao vivo, manifestos, imagens, legendas e respostas de API via CDP; é útil para sites como TikTok, Douyin, Kuaishou e Xiaohongshu que exigem uma sessão real do navegador.
- **Downloads com YT-DLP**: cole um link para analisar plataformas comuns como YouTube e Bilibili, salvar vídeo, áudio, legendas e capas, e usar uma identidade com login para baixar conteúdo que você tem permissão para acessar.
- **Transcodificação de áudio e vídeo**: baseada em FFmpeg, suporta transcodificação após o download e transcodificação de arquivos locais, com presets integrados como H.264, H.265, VP9, MP3, AAC, Opus, FLAC e WAV.

### 🗂️ Gerenciamento de Recursos

- **Gerenciamento de recursos em múltiplas visões**: as visões de tarefas e arquivos unificam downloads, transcodificações, legendas, capas e arquivos importados, com prévia, detalhes, seleção em lote, exclusão, recuperação de falhas e limpeza de registros obsoletos.

### 🎧 Player

- **Reprodução de música local**: indexa automaticamente o áudio da biblioteca e oferece fila, capas, letras sincronizadas, letras romanizadas/pinyin do Leste Asiático, equalizador e visualizações de espectro.
- **YouTube Music**: busque músicas, artistas e playlists em uma experiência de desktop, com recomendações iniciais, biblioteca de playlists, artistas seguidos, músicas curtidas, fila de reprodução e letras.
- **YouTube Live**: crie grupos e canais ao vivo personalizados, veja o status das transmissões, reproduza rádio ao vivo e abra diretamente o vídeo ao vivo.

### 🔐 Segurança e Isolamento

- **Gerenciamento automático de dependências**: instala, verifica e atualiza YT-DLP, FFmpeg, Bun e ferramentas relacionadas automaticamente; os caminhos são mantidos pelo app e não contaminam o ambiente do sistema.
- **Isolamento de credenciais e usuário**: os dados de sessões do app vêm de logins iniciados pelo usuário e são armazenados de forma independente com criptografia do sistema no macOS e no Windows; as conexões ficam separadas do navegador do dia a dia.

### 🎨 Liberdade

- **Personalização visual**: oferece pacotes de tema, modos claro/escuro/automático, cores de destaque, fontes, tamanhos de fonte e estilos de barra lateral; a Codex Pets Gallery integrada pode importar Pets online e locais.

## Prévia do Produto

<p align="center">
  <img src="./images/sniff-desk.webp" alt="Tela de captura de recursos do XiaDown" width="92%" />
  <br />
  <sub>Painel de captura para recursos de páginas web</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="Tela de reprodução do YouTube Music no XiaDown" width="92%" />
  <br />
  <sub>Reprodução de YouTube Music no desktop</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="Tela da biblioteca do XiaDown" width="92%" />
  <br />
  <sub>Biblioteca unificada para downloads e conteúdo transcodificado</sub>
</p>

<details>
  <summary><strong>Mais capturas da interface</strong></summary>

  <p align="center">
    <img src="./images/youtube-live.webp" alt="Tela de vídeo do YouTube Live no XiaDown" width="92%" />
    <br />
    <sub>Visualização de vídeo do YouTube Live</sub>
  </p>

  <p align="center">
    <img src="./images/app_sessions.webp" alt="Tela de sessões do app e status de login do XiaDown" width="92%" />
    <br />
    <sub>Sessões do app, verificação de login e gerenciamento de status da conta</sub>
  </p>

  <p align="center">
    <img src="./images/settings_download.webp" alt="Tela de configurações de download e gerenciamento de ferramentas dependentes do XiaDown" width="92%" />
    <br />
    <sub>Diretório de download, concorrência e gerenciamento de YT-DLP, FFmpeg e Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="Tela de configurações visuais do XiaDown" width="92%" />
    <br />
    <sub>Temas, modo claro/escuro, cores de destaque, fontes e tamanhos</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="Tela da Codex Pets Gallery no XiaDown" width="92%" />
    <br />
    <sub>Codex Pets Gallery e importação de Pets locais</sub>
  </p>
</details>

## Instalação

### Homebrew

No macOS, instale com Homebrew cask:

```bash
brew install --cask arnoldhao/tap/xiadown
```

### Baixar instaladores

Baixe diretamente o pacote mais recente abaixo. Versões anteriores estão disponíveis em [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Plataforma | Arquitetura | Pacote | Download |
| --- | --- | --- | --- |
| macOS | Apple Silicon | DMG | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.dmg) |
| macOS | Intel | DMG | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.dmg) |
| Windows | x64 | Instalador | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portátil | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### Primeira execução

1. `macOS`: abra o `.dmg`, arraste `XiaDown.app` para a pasta Aplicativos e abra o app.
2. `Windows`: execute diretamente o instalador `.exe`, ou descompacte o pacote portátil e abra o app. Se o SmartScreen aparecer na primeira execução, escolha `Mais informações -> Executar assim mesmo`.
3. O XiaDown abre um fluxo de boas-vindas para configurar idioma, tema, proxy e dependências. Os principais fluxos ficam na tela de boas-vindas e na interface.

### Suporte a Navegadores CDP

Navegadores compatíveis atualmente:

| Principais | Privacidade e eficiência | Especiais e regionais |
| --- | --- | --- |
| Chrome, Chromium, Edge | Brave, Vivaldi, Arc, Helium | Opera, Opera GX, Yandex Browser |

## Desenvolvimento local

Depois de preparar Go e Bun, instale a CLI do Wails 3 e inicie o modo de desenvolvimento:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.95
wails3 dev
```

## Aviso Legal

- XiaDown é fornecido como uma ferramenta auxiliar de gerenciamento de mídia e download, destinada a aprendizado, pesquisa e salvamento de conteúdo que você esteja autorizado a acessar e usar.
- Você é responsável por confirmar que qualquer download, armazenamento, conversão ou uso de conteúdo foi autorizado pelo titular dos direitos e está em conformidade com as leis aplicáveis e os termos do site/plataforma de destino.
- Não use XiaDown para baixar, distribuir, vender ou explorar de qualquer outra forma conteúdo infrator, não autorizado, pago/restrito, privado ou ilegal.
- Qualquer responsabilidade legal relacionada a direitos autorais, regras de plataforma, contas, rede ou outros assuntos decorrente do uso de XiaDown é do usuário; os mantenedores do projeto não se responsabilizam pela conduta dos usuários nem por suas consequências.

## Agradecimentos

XiaDown é construído sobre excelentes projetos open source. A experiência de desktop, o processamento de mídia, o armazenamento local, as conexões com navegadores, a música online e os recursos de interface dependem dessas bases.

| Categoria | Página inicial |
| --- | --- |
| Framework de desktop | <a href="https://go.dev/" target="_blank" rel="noreferrer">Go</a> / <a href="https://v3alpha.wails.io/" target="_blank" rel="noreferrer">Wails 3</a> / <a href="https://react.dev/" target="_blank" rel="noreferrer">React</a> |
| Processamento de mídia | <a href="https://github.com/yt-dlp/yt-dlp" target="_blank" rel="noreferrer">yt-dlp</a> / <a href="https://ffmpeg.org/" target="_blank" rel="noreferrer">FFmpeg</a> |
| Armazenamento local | <a href="https://www.sqlite.org/" target="_blank" rel="noreferrer">SQLite</a> / <a href="https://bun.uptrace.dev/" target="_blank" rel="noreferrer">Bun ORM</a> |
| Conexões com navegadores | <a href="https://chromedevtools.github.io/devtools-protocol/" target="_blank" rel="noreferrer">Chrome DevTools Protocol</a> / <a href="https://github.com/chromedp/chromedp" target="_blank" rel="noreferrer">chromedp</a> |
| Experiência frontend | <a href="https://bun.sh/" target="_blank" rel="noreferrer">Bun</a> / <a href="https://vite.dev/" target="_blank" rel="noreferrer">Vite</a> / <a href="https://lucide.dev/" target="_blank" rel="noreferrer">Lucide</a> / <a href="https://www.radix-ui.com/" target="_blank" rel="noreferrer">Radix UI</a> |

## Colaboração

- O projeto está em desenvolvimento ativo e não aceita pull requests por enquanto. Feedback, relatórios de bugs e cenários de uso são bem-vindos por [GitHub Issues](https://github.com/arnoldhao/xiadown/issues) ou e-mail.
- Este repositório é licenciado sob `Apache-2.0`. Veja [LICENSE](./LICENSE).

## Contato

- Site: <https://xiadown.app/>
- Guia de uso: <https://xiadown.app/pt-br/docs/>
- E-mail: <xunruhao@gmail.com>
