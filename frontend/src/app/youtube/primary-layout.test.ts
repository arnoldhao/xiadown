import { describe, expect, test } from "bun:test";

describe("YouTube workspace primary page layout", () => {
	test("uses the shared Browse recipe with a fixed rail and visible content heading", async () => {
		const source = await Bun.file(
			new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
		).text();
		const css = await Bun.file(
			new URL("./youtube-workspace.css", import.meta.url),
		).text();

		expect(source).toContain("<WorkspacePage");
		expect(source).toContain("<WorkspacePageTopBar");
		expect(source).toContain("<WorkspacePageContent");
		expect(source).toContain('recipe: "browse"');
		expect(source).toContain('heading: "display"');
		expect(source).toContain('topBar: "drag"');
		expect(source).toContain(
			'data-youtube-page-header={selectedPlaylist ? "playlist" : routeId}',
		);
		expect(source).not.toContain('className="youtube-workspace-header"');
		expect(css).not.toContain(".youtube-workspace-header");
		expect(css).toContain("padding: 20px 24px 24px;");
		expect(css).not.toContain(".youtube-workspace-page-heading-row");
		const pageRule =
			css.match(/\.youtube-workspace-page\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
		expect(pageRule).not.toContain("display: flex");
		expect(pageRule).toContain("container: youtube-workspace / inline-size;");
		expect(css).toContain(
			"--youtube-watch-horizontal-gap: clamp(12px, 2.5cqw, 28px);",
		);
		expect(css).not.toContain("--youtube-watch-horizontal-gap: clamp(12px, 2.5vw, 28px);");
		expect(source).toContain(
			"const transportNode = playbackState && watchDetail ? (",
		);
		expect(source).toContain("transport={transportNode}");
		expect(source).not.toContain("{presentedTransport}");
	});

	test("keeps Search centered below the shared fixed search rail", async () => {
			const [source, css, layoutContract] = await Promise.all([
				Bun.file(new URL("./YouTubeWorkspacePage.tsx", import.meta.url)).text(),
				Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
				Bun.file(new URL("../../shared/styles/dream/layout-contract.css", import.meta.url)).text(),
			]);

			expect(source).toContain('const searchPage = routeId === "search"');
			expect(source).toContain('recipe: "search"');
			expect(source).toContain('heading: "assistive"');
			expect(source).toContain('topBar: "search"');
			expect(source).toContain(
				"reserveWindowControls={reserveWindowControls}",
			);
			expect(source).toContain("<WorkspaceSearchControl");
			expect(source).toContain('from "@/shared/ui/workspace-search-control"');
			const searchHeader = source.indexOf("<WorkspacePageTopBar");
			const searchHeaderEnd = source.indexOf("</WorkspacePageTopBar>", searchHeader);
			const content = source.indexOf("<WorkspacePageContent", searchHeaderEnd);
			const searchControl = source.indexOf(
				"<WorkspaceSearchControl",
				content,
			);
			expect(content).toBeGreaterThan(searchHeaderEnd);
			expect(searchControl).toBeGreaterThan(content);
			expect(css).toContain(".youtube-workspace-scroll--search {");
			expect(css).not.toContain("width: calc(100% + 48px)");
			expect(css).not.toContain(".youtube-workspace-search");
			expect(layoutContract).toContain(".app-station-search-content-search {");
			expect(layoutContract).toContain("width: min(720px, calc(100% - 24px));");
			expect(source).toContain(
				'const searchLanding = searchPage && !submittedQuery.trim();',
			);
			expect(source).toContain("{searchLanding ? null : error ? (");
			expect(source).toContain('routeId === "search" &&');
		});

	test("gives playlist details their own title while preserving an in-content back action", async () => {
		const source = await Bun.file(
			new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
		).text();

		expect(source).toContain(
			"const pageTitle = selectedPlaylist?.title || workspaceRouteLabel(routeId, text);",
		);
		expect(source).toContain("<WorkspacePrimaryHeaderAction");
		expect(source).toContain("label={text.actions.back}");
		expect(source).not.toContain("youtube-workspace-page-back");
		expect(source).toContain('topBar: "navigation"');
		expect(source).toContain('recipe: "detail"');
		expect(source).toContain("backFromYouTubePrimaryDetail(current)");
	});

	test("uses a non-scrolling Watch header, video region, and footer without inline related content", async () => {
		const source = await Bun.file(
			new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
		).text();
		const [css, appearanceCSS] = await Promise.all([
			Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
			Bun.file(new URL("../../shared/styles/dream/youtube.css", import.meta.url)).text(),
		]);
		const watchPageRule =
			css.match(/\.youtube-workspace-watch-page\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
		const watchPageAppearanceRule =
			appearanceCSS.match(/\.youtube-workspace-watch-page\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
		const watchHeaderRule =
			css.match(/(?:^|\n)\.youtube-workspace-watch-header\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
		const watchInfoRule =
			css.match(/\.youtube-workspace-watch-info\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
		const watchBylineRule =
			css.match(/\.youtube-workspace-watch-byline\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
		const watchTitleRule =
			appearanceCSS.match(/\.youtube-workspace-watch-info > h1\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
		const videoRegionRule =
			css.match(/\.youtube-workspace-watch-video-region\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";

		expect(source).toContain(
			'className="youtube-workspace-watch-header wails-drag"',
		);
		expect(source).toContain(
			'className="youtube-workspace-watch-back wails-no-drag"',
		);
		expect(source).toContain("<h1 title={title}>{title}</h1>");
		expect(source).toContain('className="youtube-workspace-watch-uploader"');
		expect(source).toContain('className="youtube-workspace-watch-uploader-avatar"');
		expect(source).toContain('className="youtube-workspace-watch-stats"');
		expect(source).toContain('className="youtube-workspace-watch-video-region"');
		expect(source).toContain("{transport}");
		expect(source).not.toContain("YouTubeRelatedVideoRow");
		expect(source).not.toContain("youtube-workspace-watch-related");
		expect(css).not.toContain(".youtube-workspace-watch-scroll");
		expect(watchPageRule).toContain("overflow: hidden;");
		expect(watchPageAppearanceRule).toContain(
			"background: var(--app-workspace-primary-subpane-surface);",
		);
		expect(watchPageAppearanceRule).not.toContain("linear-gradient");
		expect(videoRegionRule).toContain("min-height: 0;");
		expect(videoRegionRule).toContain("flex: 1 1 auto;");
		expect(css).not.toContain("overflow-x: auto;");
		expect(source).toContain("youtube-workspace-control-error");
		expect(source).not.toContain("absolute bottom-24");
		expect(css).toContain("--youtube-workspace-footer-height: 80px;");
		expect(css).toContain("--youtube-watch-control-height: 64px;");
		expect(css).toMatch(
			/\.youtube-workspace-watch-header\s*\{[^}]*min-height:\s*var\(--youtube-watch-control-height\)/s,
		);
		expect(watchHeaderRule).toContain(
			"height: var(--youtube-watch-control-height);",
		);
		expect(watchHeaderRule).toContain(
			"padding: 11px var(--youtube-watch-horizontal-gap) 3px;",
		);
		expect(watchHeaderRule).not.toContain("border-bottom:");
		expect(watchInfoRule).toContain("grid-template-rows: 18px 24px;");
		expect(watchInfoRule).toContain("row-gap: 2px;");
		expect(watchTitleRule).toContain("font-size: clamp(13px, 1.2cqw, 15px);");
		expect(watchTitleRule).toContain("line-height: 18px;");
		expect(watchBylineRule).toContain("height: 24px;");
		expect(watchBylineRule).toContain("margin-top: 0;");
		expect(css).toContain(
			"--youtube-watch-horizontal-gap: clamp(12px, 2.5cqw, 28px);",
		);
		expect(css).toContain(
			"--youtube-watch-content-gap: clamp(10px, 1.8vh, 18px);",
		);
		expect(css).toContain("--youtube-watch-header-content-edge-gap: 8px;");
		expect(videoRegionRule).toContain("padding: calc(");
		expect(videoRegionRule).toContain("var(--youtube-watch-content-gap)");
		expect(videoRegionRule).toContain(
			"var(--youtube-watch-header-content-edge-gap)",
		);
		expect(css).not.toContain("--listen-native-video-surface:");
		expect(css).toContain(
			"bottom: calc(var(--youtube-workspace-footer-height) + 12px);",
		);
	});

	test("moves More into the Watch byline and opens a centered video info dialog", async () => {
		const [pageSource, dialogSource, css] = await Promise.all([
			Bun.file(new URL("./YouTubeWorkspacePage.tsx", import.meta.url)).text(),
			Bun.file(new URL("./YouTubeVideoInfoDialog.tsx", import.meta.url)).text(),
			Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
		]);

		expect(pageSource).toContain('className="youtube-workspace-watch-more wails-no-drag"');
		expect(pageSource).toContain('side="bottom"');
		expect(pageSource).toContain('align="center"');
		expect(pageSource).toContain('onLike={() => rateVideo("like")}');
		expect(pageSource).toContain('onDislike={() => rateVideo("dislike")}');
		expect(pageSource).toContain("rateYouTubeWorkspaceVideo(playback.videoId, next)");
		expect(pageSource).toContain("<YouTubeVideoInfoDialog");
		expect(dialogSource).toContain("details?.description?.trim()");
		expect(dialogSource).toContain("youtube-video-info-dialog-description");
		expect(dialogSource).toContain("<DialogContent");
		expect(dialogSource).toContain('variant="glass"');
		expect(dialogSource).toContain('shape="circle"');
		expect(dialogSource).toContain('title={labels.close}');
		expect(dialogSource.indexOf("youtube-video-info-dialog-hero")).toBeLessThan(
			dialogSource.indexOf("youtube-video-info-dialog-header"),
		);
		expect(dialogSource.indexOf("youtube-video-info-dialog-header")).toBeLessThan(
			dialogSource.indexOf("youtube-video-info-dialog-scroll"),
		);
		expect(pageSource).toContain('className="youtube-workspace-video-title"');

		const dialogCloseRule =
			css.match(/\.youtube-video-info-dialog-close\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
		expect(dialogCloseRule).toContain("top: 0.85rem;");
		expect(dialogCloseRule).toContain("left: 0.85rem;");
		expect(dialogCloseRule).not.toContain("right:");
		expect(dialogCloseRule).not.toMatch(/background|border|box-shadow|color/);
	});

	test("reserves primary Windows chrome only when the companion does not own it", async () => {
		const [pageSource, mainSource, css] = await Promise.all([
			Bun.file(new URL("./YouTubeWorkspacePage.tsx", import.meta.url)).text(),
			Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
			Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
		]);

		expect(mainSource).toContain(
			"reserveWindowControls={primaryWindowsChromeVisible}",
		);
		expect(pageSource).toContain(
			"reserveWindowControls={reserveWindowControls}",
		);
		expect(pageSource).toContain("data-reserve-window-controls");
		expect(css).toContain(
			'.youtube-workspace-page[data-reserve-window-controls="true"]',
		);
		expect(css).not.toMatch(
			/\.youtube-workspace-scroll\s*\{[^}]*padding-top:\s*calc\(var\(--app-windows-caption-button-height\)/s,
		);
		expect(css).toMatch(
			/\.youtube-workspace-page\[data-reserve-window-controls="true"\][\s\S]*?\.youtube-workspace-watch-header\s*\{[^}]*padding-right:\s*calc\(/,
		);
	});

	test("moves Up Next into the YouTube companion portal", async () => {
		const pageSource = await Bun.file(
			new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
		).text();
		const mainSource = await Bun.file(
			new URL("../main/MainApp.tsx", import.meta.url),
		).text();

		expect(pageSource).toContain("<YouTubeUpNextCompanion");
		expect(pageSource).toContain("upNextPortalTarget");
		expect(pageSource).toContain("createPortal(");
		expect(pageSource).toContain("onToggleUpNext={onToggleUpNext}");
		expect(mainSource).toContain('id: "youtube-up-next"');
		expect(mainSource).toContain("ref={setYouTubeUpNextPortalTarget}");
		expect(mainSource).toContain("onToggleUpNext={toggleYouTubeUpNext}");
	});

	test("cancels stale Watch transitions and reports queue navigation failures in place", async () => {
		const source = await Bun.file(
			new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
		).text();

		expect(source).toContain("const dismissWatch = React.useCallback(() => {");
		expect(source).toContain(
			"if (!preserveBackgroundRequest) {\n\t\t\tplayRequestRef.current += 1;",
		);
		expect(source).toContain("onBack={dismissWatch}");
		expect(source).toContain(
			"if (watchOpen) {\n\t\t\t  reportControlError(reason);",
		);
	});

	test("restores Watch explicitly and preserves uploader navigation until playback succeeds", async () => {
		const [pageSource, mainSource] = await Promise.all([
			Bun.file(new URL("./YouTubeWorkspacePage.tsx", import.meta.url)).text(),
			Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
		]);

		expect(mainSource).toContain(
			"setYouTubeWatchRevealRequest((request) => request + 1);",
		);
		expect(mainSource).toContain(
			"revealWatchRequest={youtubeWatchRevealRequest}",
		);
		expect(mainSource).toContain(
			"onWatchSurfaceVisibleChange={setYouTubeWatchSurfaceVisible}",
		);
		expect(pageSource).toContain(
			"onWatchSurfaceVisibleChange?.(playerSurfaceVisible)",
		);
		expect(pageSource).toContain(
			"handledRevealWatchRequestRef.current === revealWatchRequest",
		);
		expect(pageSource).toContain(
			'setUploaderTarget(null);\n\t\tsetUploaderReturnTarget(null);\n\t\tonWatchOpenChange(true);',
		);
		expect(pageSource).toContain(
			"returnToUploader: uploaderTarget,",
		);
		expect(pageSource).toContain(
			"if (revealWatch && options.returnToUploader) {\n\t\t\tsetUploaderReturnTarget(options.returnToUploader);",
		);
		expect(pageSource).toContain(
			"if (uploaderReturnTarget) {\n\t\t\tsetVideoInfoOpen(false);",
		);
		expect(pageSource).toContain("onBack={closeUploader}");
		expect(pageSource).toContain(
			"const pendingPlayRequestIDsRef = React.useRef(new Set<number>());",
		);
		expect(pageSource).toContain(
			"Array.from(pendingPlayRequestIDsRef.current)",
		);
		expect(pageSource).toContain(
			"playYouTubeWorkspaceVideo(video, backendRequestID, text.locale)",
		);
		expect(pageSource).toContain(
			"acceptYouTubeWorkspacePlay(backendRequestID)",
		);
		expect(pageSource).toContain(
			"await cancelYouTubeWorkspacePlay(requestID)",
		);
	});

	test("keeps global queue changes in the background and pops Watch before restoring Uploader", async () => {
		const [source, mainSource] = await Promise.all([
			Bun.file(new URL("./YouTubeWorkspacePage.tsx", import.meta.url)).text(),
			Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
		]);

		expect(source).toContain(
			"const revealWatch = externalCommand?.revealWatch !== false;",
		);
		expect(source).toContain(
			"allowInBackground: !revealWatch,\n\t\t\t\trevealWatch,",
		);
		expect(mainSource).toContain(
			"revealWatch: options.revealYouTube !== false,",
		);
		expect(source).toContain(
			"if (revealWatch) {\n\t\t\tonWatchOpenChange(true);",
		);
		expect(source).toContain(
			"backgroundRequest?.request === playRequestRef.current",
		);
		expect(source).toContain(
			"setPrimaryDetail((current) => backFromYouTubePrimaryDetail(current));\n\t\t\tsetUploaderTarget(uploaderReturnTarget);",
		);
		expect(source).toContain(
			"setUploaderReturnTarget(null);\n\t\t\tonWatchOpenChange(false);",
		);
	});

	test("hands Windows fullscreen to the detached native player window", async () => {
		const pageSource = await Bun.file(
			new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
		).text();
		const apiSource = await Bun.file(
			new URL("./api.ts", import.meta.url),
		).text();
		const nativeSurfaceSource = await Bun.file(
			new URL("./YouTubeNativeVideoSurface.tsx", import.meta.url),
		).text();
		const mainSource = await Bun.file(
			new URL("../main/MainApp.tsx", import.meta.url),
		).text();
		const css = await Bun.file(
			new URL("./youtube-workspace.css", import.meta.url),
		).text();

		expect(pageSource).toContain(
			"requestYouTubeEmbeddedVideoFullscreen(playback.sessionId)",
		);
		expect(pageSource).toContain(
			"subscribeYouTubeEmbeddedVideoFullscreen((fullscreen, sessionID)",
		);
		expect(pageSource).toContain(
			"playbackSessionRef.current.trim() !== sessionID",
		);
		expect(pageSource).not.toContain(
			"subscribeYouTubeEmbeddedVideoFullscreen(sessionID",
		);
		expect(pageSource).toContain(
			"geometrySuspended={embeddedFullscreen || fullscreenRequestPending}",
		);
		expect(nativeSurfaceSource).toContain("if (geometrySuspendedRef.current)");
		expect(nativeSurfaceSource).toContain("const hasVideo = Boolean(videoId)");
		expect(nativeSurfaceSource).toContain(
			"allowRemotePosterCandidates = true",
		);
		expect(nativeSurfaceSource).toContain(
			"[active, hasVideo, resetHole, scrollViewportSelector, setHole]",
		);
		expect(pageSource).toMatch(
			/if \(!sessionID \|\| !active \|\| !watchOpen\) \{\r?\n\t\t\tsetEmbeddedFullscreen\(false\);/,
		);
		expect(pageSource).not.toContain(
			"fullscreenSignalVersionRef.current += 1;\n\t\tsetEmbeddedFullscreen(false);\n\t\tsetFullscreenRequestPending(false);",
		);
		expect(pageSource).toContain("onFullscreen={toggleFullscreen}");
		expect(pageSource).toContain("fullscreenActive={embeddedFullscreen}");
		expect(pageSource).not.toContain("fullscreenModalActive");
		expect(pageSource).not.toContain("element.inert = true");
		expect(pageSource).not.toContain(
			'data-video-app-fullscreen={embeddedFullscreen ? "true" : undefined}',
		);
		expect(css).not.toContain(
			'.youtube-workspace-page[data-video-app-fullscreen="true"]',
		);
		expect(apiSource).toContain('"RequestEmbeddedVideoFullscreen"');
		expect(apiSource).toContain('"embedded-video-fullscreen-change"');
		expect(apiSource).toContain('payload.provider === "youtube"');
		expect(pageSource).not.toContain("fullscreenPortalTarget");
		expect(pageSource).not.toContain("playerPresentation");
		expect(mainSource).not.toContain("youtubeFullscreenPortalTarget");
		expect(mainSource).not.toContain('setFullscreenPlayer("youtube")');
		expect(mainSource).toContain('"music" | null');
	});

	test("uses semantic media glass without raw blur or global numeric layers", async () => {
		const [css, uploaderCSS, appearanceCSS, pageSource, nativeSurfaceSource, workflowsCSS] =
			await Promise.all([
				Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
				Bun.file(new URL("./youtube-uploader-page.css", import.meta.url)).text(),
				Bun.file(new URL("../../shared/styles/dream/youtube.css", import.meta.url)).text(),
				Bun.file(new URL("./YouTubeWorkspacePage.tsx", import.meta.url)).text(),
				Bun.file(new URL("./YouTubeNativeVideoSurface.tsx", import.meta.url)).text(),
				Bun.file(new URL("../../shared/styles/dream/workflows.css", import.meta.url)).text(),
			]);
		const cardLoadingRule = css.match(
			/\.youtube-workspace-card-loading\s*\{([\s\S]*?)\n\}/,
		)?.[1];
		const playerErrorRule = css.match(
			/\.youtube-workspace-player-error\s*\{([\s\S]*?)\n\}/,
		)?.[1];

		expect(cardLoadingRule).not.toMatch(/background|border|box-shadow|color/);
		expect(playerErrorRule).not.toMatch(/background|border|box-shadow|color/);
		expect(`${css}\n${uploaderCSS}\n${appearanceCSS}`).not.toMatch(
			/(?:-webkit-)?backdrop-filter\s*:/,
		);
		expect(`${css}\n${uploaderCSS}\n${appearanceCSS}`).not.toMatch(
			/(?:color|border-color|background|fill):[^;]*(?:\bwhite\b|rgb\(255 255 255|#fff(?:fff)?\b)/,
		);
		expect(appearanceCSS).toContain("var(--app-media-chrome-foreground)");
		expect(pageSource).toContain('surfaceRole="status"');
		expect(pageSource).toContain("app-dream-status-message");
		expect(pageSource).toContain("<StatusBadge");
		expect(nativeSurfaceSource).toContain('surfaceRole="status"');
		expect(workflowsCSS).toContain(
			'.youtube-workspace-player-error[data-surface-role="status"]',
		);
		expect(css).not.toMatch(
			/z-index:\s*(?:[2-9]\d|[1-9]\d{2,})\s*;/,
		);
	});

	test("maps every sidebar route to a large-page label", async () => {
		const source = await Bun.file(
			new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
		).text();
		const labelResolver = source.slice(
			source.indexOf("function workspaceRouteLabel"),
			source.indexOf("function readErrorMessage"),
		);

		for (const routeId of [
			"search",
			"subscriptions",
			"explore",
			"shorts",
			"liked-videos",
			"watch-later",
			"playlists",
			"history",
		]) {
			expect(labelResolver).toContain(`case "${routeId}":`);
		}
		expect(labelResolver).toContain("return text.workspace.home;");
	});
});
