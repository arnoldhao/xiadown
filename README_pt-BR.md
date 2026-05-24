<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ícone do XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Uma ferramenta de download de vídeo com suporte a música online.</strong></p>
  <p>Listen Keep, Make it Yours</p>
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
  <p>
    <img src="https://img.shields.io/github/v/tag/arnoldhao/xiadown?label=version" alt="Versão mais recente" />
    <img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="Licença" />
    <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS-lightgrey" alt="Plataformas compatíveis" />
    <img src="https://img.shields.io/badge/stack-Go%20%E2%80%A2%20Wails%20%E2%80%A2%20React-green" alt="Stack tecnológico" />
  </p>
</div>

## Visão Geral

XiaDown é um reprodutor de música online e também uma ferramenta de download de vídeo.

Ele foi criado para criadores de conteúdo: quando você precisa de material, oferece downloads potentes com base no YT-DLP; quando precisa trabalhar, mantém a música online tocando em segundo plano. Com pets e personalização visual, o app continua simples sem ficar sem graça.

## Principais Recursos

- **Reprodutor de música online**: um player de desktop feito para estações Lo-Fi do YouTube e YouTube Music, com login de conta, busca de músicas, artistas e playlists, fila de reprodução, letras, capas e suporte a estações Lo-Fi online personalizadas. Faixas que valem guardar podem ser baixadas para a biblioteca local.
- **Downloads de vídeo e áudio**: com YT-DLP, oferece suporte a downloads de material de milhares de sites de vídeo online; cole um link para salvar vídeo, áudio, legendas e capas, depois transcodifique e gerencie tudo na biblioteca local.
- **Espaço de mídia personalizado**: pacotes de tema cuidadosamente desenhados, cores de destaque, modos de aparência, estilos de barra lateral e suporte completo a Codex Pets, com dependências e atualizações do app mantidas automaticamente para uso diário a longo prazo.

## Prévia do Produto

<p align="center">
  <img src="./images/download.png" alt="Tela de tarefas de download do XiaDown" width="88%" />
</p>

<p align="center">
  <img src="./images/listen.png" alt="Tela Listen de reprodução de música online do XiaDown" width="88%" />
</p>

<p align="center">
  <img src="./images/library.png" alt="Tela da biblioteca do XiaDown" width="88%" />
</p>

## Início Rápido

### Baixar e instalar

Baixe diretamente o instalador mais recente abaixo. Versões anteriores estão disponíveis em [GitHub Releases](https://github.com/arnoldhao/xiadown/releases).

| Plataforma | Arquitetura | Pacote | Download |
| --- | --- | --- | --- |
| macOS | Apple Silicon | Arquivo compactado | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-arm64-latest.zip) |
| macOS | Intel | Arquivo compactado | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-macos-x64-latest.zip) |
| Windows | x64 | Instalador | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest-installer.exe) |
| Windows | x64 | Portátil | [Download](https://updates.dreamapp.cc/xiadown/downloads/xiadown-windows-x64-latest.zip) |

### Primeira execução

1. `macOS`: descompacte o pacote e mova `XiaDown.app` para a pasta Aplicativos. Se o macOS disser que o app não pode ser aberto ou está danificado, execute `sudo xattr -rd com.apple.quarantine /Applications/XiaDown.app` no terminal.
2. `Windows`: execute diretamente o instalador `.exe`, ou descompacte o pacote portátil e abra o app. Se o SmartScreen aparecer na primeira execução, escolha `Mais informações -> Executar assim mesmo`.
3. O XiaDown abre um fluxo de boas-vindas para configurar idioma, tema, proxy e dependências. Os principais fluxos ficam na tela de boas-vindas e na interface.

## Aviso Legal

- XiaDown é fornecido como uma ferramenta auxiliar de gerenciamento de mídia e download, destinada a aprendizado, pesquisa e salvamento de conteúdo que você esteja autorizado a acessar e usar.
- Você é responsável por confirmar que qualquer download, armazenamento, conversão ou uso de conteúdo foi autorizado pelo titular dos direitos e está em conformidade com as leis aplicáveis e os termos do site/plataforma de destino.
- Não use XiaDown para baixar, distribuir, vender ou explorar de qualquer outra forma conteúdo infrator, não autorizado, pago/restrito, privado ou ilegal.
- Qualquer responsabilidade legal relacionada a direitos autorais, regras de plataforma, contas, rede ou outros assuntos decorrente do uso de XiaDown é do usuário; os mantenedores do projeto não se responsabilizam pela conduta dos usuários nem por suas consequências.

## Agradecimentos

XiaDown é construído sobre excelentes projetos open source. A experiência de desktop, o processamento de mídia, o armazenamento local, as conexões com navegadores, a música online e a interface frontend dependem dessas bases.

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

- Site: <https://xiadown.dreamapp.cc/>
- E-mail: <xunruhao@gmail.com>
