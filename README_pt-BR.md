<div align="center">
  <img src="./frontend/public/appicon.png" width="112" alt="Ícone do XiaDown" />
  <h1>XiaDown</h1>
  <p><strong>Uma ferramenta de download de vídeo com dois mecanismos e suporte a música online.</strong></p>
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

XiaDown é um reprodutor de música online e também uma ferramenta de download de vídeo com dois mecanismos.

Ele foi criado para criadores de conteúdo: quando você precisa de material, oferece downloads potentes por captura de recursos no navegador e YT-DLP; quando precisa trabalhar, mantém a música online tocando em segundo plano. Com biblioteca, transcodificação, gerenciamento automático de dependências, isolamento de contas, pets e personalização visual, o XiaDown serve tanto para lidar com material de mídia quanto para se tornar sua ferramenta de mídia de desktop no dia a dia.

## Principais Recursos

- 📥 **Download de vídeo com dois mecanismos**: inclui um mecanismo de download YT-DLP e um mecanismo de captura de navegador baseado em CDP. Links comuns podem ser analisados e baixados diretamente, inclusive usando Cookies salvos nos perfis de conexão; em páginas com carregamento dinâmico, estruturas de site complexas ou recursos que exigem uma sessão real do navegador, o modo de captura identifica vídeo, áudio, legendas, capas e outros recursos de mídia, depois segue para transcodificação e gerenciamento na biblioteca.
- 🎧 **Reprodutor de música de desktop**: gerencia automaticamente o áudio local dos recursos baixados, permite buscar músicas, artistas e playlists no YouTube Music, e reproduz rádios do YouTube Live com visualização de vídeo ao vivo. O player oferece fila, capas, letras sincronizadas, letras romanizadas/pinyin do Leste Asiático, equalizador e visualizações de espectro.
- 🧩 **Um espaço livre e sob controle**: ferramentas dependentes são instaladas e atualizadas automaticamente sem poluir o ambiente do sistema; contas, Cookies e Profiles do navegador são gerenciados por conexões isoladas; temas, modo claro/escuro, cores de destaque, fontes, tamanhos de fonte e Codex Pets podem ser ajustados livremente.

## Capacidades Essenciais

- **Download por captura de recursos**: capacidade própria de captura de navegador baseada em CDP, capaz de observar vídeo, áudio, transmissões ao vivo, manifestos, imagens, legendas, respostas de API e outros recursos de uma página. Em um ambiente real de navegador no qual o usuário fez login explicitamente, ela pode identificar e baixar recursos de TikTok, Douyin, Kuaishou, Xiaohongshu e sites semelhantes, além de encaminhar o download diretamente para a transcodificação.
- **Downloads com YT-DLP**: integra YT-DLP para baixar material de uma ampla variedade de sites de vídeo online, com suporte estável para plataformas comuns como YouTube e Bilibili. Cole um link para analisar e salvar vídeo, áudio, legendas e capas; também é possível usar Cookies salvos nos perfis de conexão para conteúdo ao qual o usuário tem autorização de acesso, depois seguir para transcodificação e gerenciamento na biblioteca.
- **Transcodificação de áudio e vídeo**: baseada em FFmpeg, com suporte a transcodificação após o download e seleção manual de arquivos locais. Inclui presets para H.264, H.265, VP9, MP3, AAC, Opus, FLAC, WAV e saídas comuns como tamanho original, 2160p, 1080p, 720p e 480p.
- **Gerenciamento de recursos em múltiplas visões**: as visões de tarefas e arquivos unificam downloads, transcodificações, legendas, capas e arquivos importados. Oferece prévia de mídia, detalhes de tarefas, detalhes de arquivos, seleção em lote, exclusão, recuperação de tarefas com falha, verificação de existência de arquivos e limpeza de registros obsoletos.
- **Reprodução de música local**: indexa automaticamente os arquivos de áudio na biblioteca, com reprodução local, fila, capas, letras sincronizadas, letras romanizadas/pinyin do Leste Asiático, equalizador e vários estilos de visualização de espectro.
- **YouTube Music**: oferece uma experiência de YouTube Music no desktop, com conexões de conta, busca de músicas/artistas/playlists, recomendações na página inicial, biblioteca de playlists, artistas seguidos, músicas curtidas, fila de reprodução, letras e limpeza de dados de anúncios para reduzir interrupções.
- **YouTube Live**: permite adicionar grupos e canais personalizados do YouTube Live, ver o status das transmissões, reproduzir rádios ao vivo e assistir diretamente ao vídeo ao vivo.
- **Gerenciamento automático de dependências**: mantém automaticamente a instalação, verificação e atualização de YT-DLP, FFmpeg, Bun e ferramentas relacionadas. Os caminhos das ferramentas são gerenciados pelo app de forma independente, sem depender do ambiente global do usuário nem poluí-lo.
- **Isolamento de credenciais e usuário**: permite usar recursos do navegador local por CDP e persistir Profiles e Cookies independentes. Os dados vêm apenas de logins iniciados pelo usuário, e as configurações de conexão ficam separadas do uso cotidiano do navegador.
- **Personalização visual**: oferece pacotes de tema, modos claro/escuro/automático, cores de destaque, fontes, tamanhos de fonte, estilos de barra lateral e muito mais. A Codex Pets Gallery integrada permite importar Pets online e locais como elementos de companhia no desktop.

## Prévia do Produto

<p align="center">
  <img src="./images/download.webp" alt="Tela de tarefas de download do XiaDown" width="88%" />
  <br />
  <sub>Tarefas de download e transcodificação</sub>
</p>

<p align="center">
  <img src="./images/sniff-desk.webp" alt="Tela de captura de recursos do XiaDown" width="88%" />
  <br />
  <sub>Painel de captura para recursos de páginas web</sub>
</p>

<p align="center">
  <img src="./images/youtube-music.webp" alt="Tela de reprodução do YouTube Music no XiaDown" width="88%" />
  <br />
  <sub>Reprodução de YouTube Music no desktop</sub>
</p>

<p align="center">
  <img src="./images/youtube-live.webp" alt="Tela de vídeo do YouTube Live no XiaDown" width="88%" />
  <br />
  <sub>Visualização de vídeo do YouTube Live</sub>
</p>

<p align="center">
  <img src="./images/library.webp" alt="Tela da biblioteca do XiaDown" width="88%" />
  <br />
  <sub>Biblioteca unificada para downloads e conteúdo transcodificado</sub>
</p>

<details>
  <summary>Mais telas de configuração e personalização</summary>

  <p align="center">
    <img src="./images/connector.webp" alt="Tela de conexões e isolamento de contas do XiaDown" width="88%" />
    <br />
    <sub>Isolamento de conexões, Cookies e Profiles do navegador</sub>
  </p>

  <p align="center">
    <img src="./images/tools.webp" alt="Tela de gerenciamento de ferramentas dependentes do XiaDown" width="88%" />
    <br />
    <sub>Gerenciamento automático de YT-DLP, FFmpeg e Bun</sub>
  </p>

  <p align="center">
    <img src="./images/appearance.webp" alt="Tela de configurações visuais do XiaDown" width="88%" />
    <br />
    <sub>Temas, modo claro/escuro, cores de destaque, fontes e tamanhos</sub>
  </p>

  <p align="center">
    <img src="./images/codex-pets-gallery.webp" alt="Tela da Codex Pets Gallery no XiaDown" width="88%" />
    <br />
    <sub>Codex Pets Gallery e importação de Pets locais</sub>
  </p>
</details>

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
