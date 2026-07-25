import * as React from "react";
import { createPortal } from "react-dom";
import {
	AlertCircle,
	ArrowLeft,
	CalendarDays,
	Download,
	ExternalLink,
	Eye,
	Info,
	LoaderCircle,
	LogIn,
	MoreHorizontal,
	ThumbsDown,
	ThumbsUp,
	UserRound,
	Youtube,
} from "lucide-react";

import { ListenInfiniteScrollSentinel } from "@/app/main/listen/infinite-scroll-sentinel";
import {
	LISTEN_YOUTUBE_REGION_UNAVAILABLE_ERROR_CODE,
	LISTEN_YOUTUBE_VERIFICATION_REQUIRED_ERROR_CODE,
} from "@/app/main/listen/catalog";
import {
	acceptYouTubeWorkspacePlay,
	appendYouTubeWorkspaceBrowsePage,
  browseYouTubeWorkspace,
	cancelYouTubeWorkspacePlay,
	consumeYouTubeExternalCommand,
	createYouTubeWorkspaceBrowseRequest,
  getYouTubeWorkspaceVideoDetails,
  getYouTubeWorkspacePlayerStatus,
  isYouTubePlayerStatusForSession,
	exitYouTubeEmbeddedVideoFullscreen,
	rateYouTubeWorkspaceVideo,
  requestYouTubeEmbeddedVideoFullscreen,
  pauseYouTubeWorkspaceVideo,
  playYouTubeWorkspaceVideo,
  resumeYouTubeWorkspaceVideo,
	seekYouTubeWorkspaceVideo,
	selectYouTubeWorkspaceAudioTrack,
	selectYouTubeWorkspaceCaption,
	selectYouTubeWorkspacePlaybackRate,
	selectYouTubeWorkspaceQuality,
	shouldCommitYouTubeVideoOpen,
  setYouTubeWorkspaceVolume,
	setYouTubeWorkspaceChannelSubscription,
  subscribeYouTubeEmbeddedVideoFullscreen,
  subscribeYouTubePlayerStatus,
	toggleYouTubeWorkspaceCaptions,
} from "@/app/youtube/api";
import {
  backFromYouTubePrimaryDetail,
  createYouTubePrimaryDetail,
  formatYouTubePublishedLabel,
  formatYouTubeViewCount,
  openYouTubePrimaryPlaylist,
  openYouTubePrimaryWatch,
  resetYouTubePrimaryDetailForRoute,
  resolveYouTubePreferredPlaybackTitle,
  resolveYouTubeVolumeCapability,
  resolveYouTubeWorkspaceErrorMessage,
  rollbackYouTubeOptimisticMute,
  submitYouTubeSearchQuery,
  type YouTubePrimaryDetail,
  updateYouTubeSubmittedQueryOnInput,
} from "@/app/youtube/page-state";
import {
	createYouTubeBrowseUploaderTarget,
	type YouTubeUploaderTarget,
	youtubeChannelURL,
} from "@/app/youtube/uploader-navigation";
import {
	reconcileYouTubeSubscriptionDetails,
	resolveYouTubeUploaderSubscriptionSync,
	YouTubeSubscriptionRequestGate,
	youtubeSubscriptionIdentity,
} from "@/app/youtube/subscription-request-gate";
import { YouTubeWorkspaceTransportBar } from "@/app/youtube/YouTubeWorkspaceTransportBar";
import { YouTubeNativeVideoSurface } from "@/app/youtube/YouTubeNativeVideoSurface";
import { YouTubeUpNextCompanion } from "@/app/youtube/YouTubeUpNextCompanion";
import { YouTubeImage } from "@/app/youtube/YouTubeImage";
import { YouTubeSubscriptionIconButton } from "@/app/youtube/YouTubeSubscriptionIconButton";
import { YouTubeUploaderPage } from "@/app/youtube/YouTubeUploaderPage";
import {
	YouTubeVideoInfoDialog,
	type YouTubeVideoInfoLabels,
} from "@/app/youtube/YouTubeVideoInfoDialog";
import type {
  YouTubePlaybackDescriptor,
  YouTubePlayerStatus,
  YouTubeVideoDetails,
  YouTubeVideoRating,
  YouTubeWorkspaceBrowsePage,
	YouTubeWorkspaceExternalCommand,
  YouTubeWorkspacePlaybackState,
  YouTubeWorkspaceRouteId,
  YouTubeWorkspaceVideo,
} from "@/app/youtube/types";
import { getXiaText } from "@/features/xiadown/shared";
import { openExternalURL } from "@/shared/query/system";
import { Button } from "@/shared/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { GlassSurface } from "@/shared/ui/glass-surface";
import { StatusBadge } from "@/shared/ui/status-badge";
import { WorkspacePrimaryHeaderAction } from "@/shared/ui/workspace-primary-header-action";
import {
	defineWorkspacePageContract,
	WorkspacePage,
	WorkspacePageContent,
	WorkspacePageTopBar,
} from "@/shared/ui/workspace-page";
import { WorkspaceSearchControl } from "@/shared/ui/workspace-search-control";

import "./youtube-workspace.css";

export { isYouTubeNativeSurfaceRectVisible } from "@/app/youtube/YouTubeNativeVideoSurface";

export interface YouTubeWorkspacePageProps {
  active?: boolean;
  routeId: string;
  text: ReturnType<typeof getXiaText>;
  watchOpen: boolean;
  revealWatchRequest?: number;
  onWatchOpenChange: (open: boolean) => void;
  onWatchSurfaceVisibleChange?: (visible: boolean) => void;
  upNextPortalTarget?: HTMLElement | null;
  upNextOpen?: boolean;
  onToggleUpNext?: () => void;
  onDownload?: (url: string) => void;
  initialPlayback?: YouTubeWorkspacePlaybackState | null;
  onPlaybackChange?: (playback: YouTubeWorkspacePlaybackState | null) => void;
	externalCommand?: YouTubeWorkspaceExternalCommand | null;
	reserveWindowControls?: boolean;
}

interface YouTubeVideoOpenOptions {
	allowInBackground?: boolean;
	revealWatch?: boolean;
	returnToUploader?: YouTubeUploaderTarget;
}

export function YouTubeWorkspacePage({
  active = true,
  routeId: rawRouteId,
  text,
  watchOpen,
  revealWatchRequest = 0,
  onWatchOpenChange,
  onWatchSurfaceVisibleChange,
  upNextPortalTarget = null,
  upNextOpen = false,
  onToggleUpNext,
  onDownload,
  initialPlayback = null,
	onPlaybackChange,
	externalCommand = null,
	reserveWindowControls = false,
}: YouTubeWorkspacePageProps) {
  const routeId = normalizeWorkspaceRoute(rawRouteId);
  const [queryInput, setQueryInput] = React.useState("");
  const [submittedQuery, setSubmittedQuery] = React.useState("");
  const [reloadToken, setReloadToken] = React.useState(0);
  const [page, setPage] = React.useState<YouTubeWorkspaceBrowsePage | null>(null);
  const [loading, setLoading] = React.useState(false);
	const [appending, setAppending] = React.useState(false);
  const [error, setError] = React.useState("");
	const [controlError, setControlError] = React.useState("");
	const [appendError, setAppendError] = React.useState("");
  const [openingVideoId, setOpeningVideoId] = React.useState("");
  const [primaryDetail, setPrimaryDetail] = React.useState<YouTubePrimaryDetail>(
    () => createYouTubePrimaryDetail(routeId),
  );
	const [uploaderTarget, setUploaderTarget] =
		React.useState<YouTubeUploaderTarget | null>(null);
	const [uploaderReturnTarget, setUploaderReturnTarget] =
		React.useState<YouTubeUploaderTarget | null>(null);
  const [playback, setPlayback] = React.useState<YouTubePlaybackDescriptor | null>(
    initialPlayback?.descriptor ?? null,
  );
  const [playerStatus, setPlayerStatus] = React.useState<YouTubePlayerStatus>(
    initialPlayback?.status ?? {},
  );
  const [queue, setQueue] = React.useState<YouTubeWorkspaceVideo[]>(
    initialPlayback?.queue ?? [],
  );
  const [currentIndex, setCurrentIndex] = React.useState(
    initialPlayback?.currentIndex ?? -1,
  );
	const [volume, setVolume] = React.useState(initialPlayback?.volume ?? 1);
  const [muted, setMuted] = React.useState(initialPlayback?.muted ?? false);
	const [videoDetails, setVideoDetails] =
		React.useState<YouTubeVideoDetails | null>(null);
	const [videoRating, setVideoRating] =
		React.useState<YouTubeVideoRating | null>(null);
	const [videoSubscriptionOverride, setVideoSubscriptionOverride] =
		React.useState<{ identity: string; value: boolean } | null>(null);
	const [subscriptionBusy, setSubscriptionBusy] = React.useState(false);
	const [videoInfoOpen, setVideoInfoOpen] = React.useState(false);
	const videoDetailsCacheRef = React.useRef(new Map<string, YouTubeVideoDetails>());
	const videoDetailsRequestRef = React.useRef(0);
	const videoRatingRequestRef = React.useRef(0);
	const videoSubscriptionGateRef = React.useRef(
		new YouTubeSubscriptionRequestGate(),
	);
	const [embeddedFullscreen, setEmbeddedFullscreen] = React.useState(false);
	const [fullscreenRequestPending, setFullscreenRequestPending] =
		React.useState(false);
	const fullscreenSignalVersionRef = React.useRef(0);
	const handledExternalCommandID = React.useRef<number | null>(null);
	const stoppedPlaybackSessionIDRef = React.useRef("");
	const handledRevealWatchRequestRef = React.useRef(revealWatchRequest);
	const browseGenerationRef = React.useRef(0);
	const requestedContinuationsRef = React.useRef(new Set<string>());
	const previousRouteRef = React.useRef(routeId);
	const browseScrollRef = React.useRef<HTMLDivElement | null>(null);
	const fullscreenButtonRef = React.useRef<HTMLButtonElement | null>(null);
	const browseScrollTopRef = React.useRef(0);
	const playRequestRef = React.useRef(0);
	const pendingPlayRequestIDsRef = React.useRef(new Set<number>());
	const backgroundPlayRequestRef = React.useRef<{
		request: number;
		backendRequestID: number;
	} | null>(null);
	const muteRequestRef = React.useRef(0);
	const volumeWriteTimerRef = React.useRef<number | null>(null);
	const desiredVolumeRef = React.useRef<{
		sessionId: string;
		volume: number;
		muted: boolean;
	} | null>(null);
	const playbackSessionRef = React.useRef(playback?.sessionId ?? "");
	const activeRef = React.useRef(active);
	const routeRef = React.useRef(routeId);
	activeRef.current = active;
	routeRef.current = routeId;
	playbackSessionRef.current = playback?.sessionId ?? "";
	React.useEffect(() => () => {
		videoSubscriptionGateRef.current.invalidate();
	}, []);
	const selectedPlaylist = React.useMemo(() => {
		if (primaryDetail.kind === "playlist") {
			return primaryDetail.playlist;
		}
		if (
			primaryDetail.kind === "watch" &&
			primaryDetail.returnTarget.kind === "playlist"
		) {
			return primaryDetail.returnTarget.playlist;
		}
		return null;
	}, [primaryDetail]);
	const invalidateVideoOpenRequests = React.useCallback(() => {
		const backgroundRequest = backgroundPlayRequestRef.current;
		const preserveBackgroundRequest =
			backgroundRequest?.request === playRequestRef.current;
		const backgroundBackendRequestID = preserveBackgroundRequest
			? backgroundRequest?.backendRequestID
			: undefined;
		if (!preserveBackgroundRequest) {
			playRequestRef.current += 1;
			backgroundPlayRequestRef.current = null;
		}
		setOpeningVideoId("");
		const requestIDs = Array.from(pendingPlayRequestIDsRef.current)
			.filter((requestID) => requestID !== backgroundBackendRequestID)
			.reverse();
		pendingPlayRequestIDsRef.current = new Set(
			backgroundBackendRequestID === undefined
				? []
				: [backgroundBackendRequestID],
		);
		if (requestIDs.length === 0) {
			return;
		}
		void (async () => {
			for (const requestID of requestIDs) {
				try {
					await cancelYouTubeWorkspacePlay(requestID);
				} catch {
					// Cancellation is best-effort when the App is already closing.
				}
			}
		})();
	}, []);
	const reportControlError = React.useCallback(
		(reason: unknown) => {
			const message = readErrorMessage(reason, text, "control");
			if (message) {
				setControlError(message);
			}
		},
		[text],
	);
	const dismissWatch = React.useCallback(() => {
		invalidateVideoOpenRequests();
		if (uploaderReturnTarget) {
			setVideoInfoOpen(false);
			setPrimaryDetail((current) => backFromYouTubePrimaryDetail(current));
			setUploaderTarget(uploaderReturnTarget);
			setUploaderReturnTarget(null);
			onWatchOpenChange(false);
			return;
		}
		setPrimaryDetail((current) => backFromYouTubePrimaryDetail(current));
		onWatchOpenChange(false);
	}, [
		invalidateVideoOpenRequests,
		onWatchOpenChange,
		uploaderReturnTarget,
	]);
	const clearStoppedPlayback = React.useCallback(() => {
		const stoppedSessionID = playbackSessionRef.current.trim();
		if (stoppedSessionID) {
			stoppedPlaybackSessionIDRef.current = stoppedSessionID;
		}
		playbackSessionRef.current = "";
		desiredVolumeRef.current = null;
		fullscreenSignalVersionRef.current += 1;
		invalidateVideoOpenRequests();
		setPlayback(null);
		setPlayerStatus({});
		setQueue([]);
		setCurrentIndex(-1);
		setEmbeddedFullscreen(false);
		setFullscreenRequestPending(false);
		setVideoInfoOpen(false);
		setControlError("");
		setUploaderReturnTarget(null);
		setPrimaryDetail((current) =>
			current.kind === "watch"
				? backFromYouTubePrimaryDetail(current)
				: current,
		);
		onWatchOpenChange(false);
	}, [invalidateVideoOpenRequests, onWatchOpenChange]);

	React.useEffect(() => {
		if (!controlError) {
			return;
		}
		const timeout = window.setTimeout(() => setControlError(""), 5000);
		return () => window.clearTimeout(timeout);
	}, [controlError]);

  React.useEffect(() => {
    const restored = initialPlayback;
    const restoredSessionID = restored?.descriptor.sessionId?.trim() || "";
    if (
      !restored ||
      !restoredSessionID ||
      restoredSessionID === playback?.sessionId ||
      restoredSessionID === stoppedPlaybackSessionIDRef.current
    ) {
      return;
    }
    stoppedPlaybackSessionIDRef.current = "";
    setPlayback(restored.descriptor);
    setPlayerStatus(restored.status);
    setQueue(restored.queue);
    setCurrentIndex(restored.currentIndex);
    setVolume(restored.volume);
    setMuted(restored.muted);
    const video = playbackDescriptorToVideo(
      restored.descriptor,
      restored.status,
    );
    setPrimaryDetail((current) => openYouTubePrimaryWatch(current, video));
    onWatchOpenChange(true);
  }, [initialPlayback, onWatchOpenChange, playback?.sessionId]);

  React.useEffect(() => {
    const sessionID = playback?.sessionId?.trim();
    if (!sessionID) {
      return;
    }
	return subscribeYouTubePlayerStatus(sessionID, (status) => {
		setPlayerStatus(status);
		const desired = desiredVolumeRef.current;
		const statusVolume = Number.isFinite(status.volume)
			? Math.max(0, Math.min(1, Number(status.volume)))
			: null;
		const statusMuted = typeof status.muted === "boolean" ? status.muted : null;
		const desiredAcknowledged = Boolean(
			desired &&
			desired.sessionId === sessionID &&
			(statusVolume === null || Math.abs(statusVolume - desired.volume) < 0.005) &&
			(statusMuted === null || statusMuted === desired.muted),
		);
		if (!desired || desired.sessionId !== sessionID || desiredAcknowledged) {
			if (statusVolume !== null) setVolume(statusVolume);
			if (statusMuted !== null) setMuted(statusMuted);
		}
		if (desiredAcknowledged) desiredVolumeRef.current = null;
	});
  }, [playback?.sessionId]);

	React.useEffect(() => {
		return subscribeYouTubeEmbeddedVideoFullscreen((fullscreen, sessionID) => {
			if (!sessionID || playbackSessionRef.current.trim() !== sessionID) {
				return;
			}
			fullscreenSignalVersionRef.current += 1;
			setEmbeddedFullscreen(fullscreen);
			setFullscreenRequestPending(false);
		});
	}, []);

	React.useEffect(() => {
		fullscreenSignalVersionRef.current += 1;
		setFullscreenRequestPending(false);
		const sessionID = playback?.sessionId?.trim();
		if (!sessionID || !active || !watchOpen) {
			setEmbeddedFullscreen(false);
		}
	}, [active, playback?.sessionId, watchOpen]);

	React.useEffect(() => {
		videoRatingRequestRef.current += 1;
		setVideoRating(null);
	}, [playback?.sessionId]);

  React.useEffect(() => {
    if (!playback) {
      return;
    }
    let cancelled = false;
    void getYouTubeWorkspacePlayerStatus()
      .then((status) => {
        if (
          !cancelled &&
          status?.videoId === playback.videoId &&
          isYouTubePlayerStatusForSession(status, playback.sessionId)
        ) {
          setPlayerStatus(status);
		  if (Number.isFinite(status.volume)) {
			setVolume(Math.max(0, Math.min(1, Number(status.volume))));
		  }
		  if (typeof status.muted === "boolean") {
			setMuted(status.muted);
		  }
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
	}, [playback?.sessionId, playback?.videoId]);

  const playbackState = React.useMemo<YouTubeWorkspacePlaybackState | null>(() => {
    if (!playback) {
      return null;
    }
    return {
      descriptor: playback,
      status: playerStatus,
      currentIndex,
      queue,
      muted,
      volume,
      capabilities: {
        previous: currentIndex > 0,
        next: currentIndex >= 0 && currentIndex + 1 < queue.length,
        playPause: true,
		like: playerStatus.controls?.like === true,
		dislike: playerStatus.controls?.dislike === true,
        fullscreen: true,
		captions: playerStatus.controls?.captions === true,
		audioTrack: playerStatus.controls?.audioTrack === true,
		quality: playerStatus.controls?.quality === true,
		volume: resolveYouTubeVolumeCapability(playback, playerStatus),
		playbackRate: playerStatus.controls?.playbackRate === true,
      },
    };
  }, [currentIndex, muted, playback, playerStatus, queue, volume]);

  React.useEffect(() => {
    onPlaybackChange?.(playbackState);
  }, [onPlaybackChange, playbackState]);

	React.useEffect(() => {
		const changed = previousRouteRef.current !== routeId;
		previousRouteRef.current = routeId;
		setPrimaryDetail((current) =>
			resetYouTubePrimaryDetailForRoute(current, routeId),
		);
		if (changed) {
			invalidateVideoOpenRequests();
			setUploaderTarget(null);
			setUploaderReturnTarget(null);
			onWatchOpenChange(false);
		}
	}, [invalidateVideoOpenRequests, onWatchOpenChange, routeId]);

	React.useLayoutEffect(() => {
		if (!active) {
			invalidateVideoOpenRequests();
		}
	}, [active, invalidateVideoOpenRequests]);

	React.useEffect(() => {
		if (watchOpen) {
			return;
		}
		invalidateVideoOpenRequests();
		setUploaderReturnTarget(null);
	}, [invalidateVideoOpenRequests, watchOpen]);

	React.useLayoutEffect(() => {
		if (handledRevealWatchRequestRef.current === revealWatchRequest) {
			return;
		}
		handledRevealWatchRequestRef.current = revealWatchRequest;
		invalidateVideoOpenRequests();
		setUploaderTarget(null);
		setUploaderReturnTarget(null);
		onWatchOpenChange(true);
	}, [invalidateVideoOpenRequests, onWatchOpenChange, revealWatchRequest]);

	React.useEffect(() => {
		if (!watchOpen) {
			setPrimaryDetail((current) =>
				current.kind === "watch"
					? backFromYouTubePrimaryDetail(current)
					: current,
			);
			return;
		}
		if (!playback) {
			return;
		}
		const queueVideo = queue.find(
			(item) => item.videoId === playback.videoId,
		);
		const video = queueVideo ?? playbackDescriptorToVideo(playback, playerStatus);
		setPrimaryDetail((current) => {
			if (
				current.kind === "watch" &&
				current.video.videoId === video.videoId
			) {
				return current;
			}
			return openYouTubePrimaryWatch(current, video);
		});
	}, [playback, playerStatus, queue, watchOpen]);

	const playerSurfaceVisible =
		active && watchOpen && primaryDetail.kind === "watch" && !uploaderTarget;

	React.useLayoutEffect(() => {
		onWatchSurfaceVisibleChange?.(playerSurfaceVisible);
	}, [onWatchSurfaceVisibleChange, playerSurfaceVisible]);

  React.useEffect(() => {
    let active = true;
    let unsubscribe = () => {};
    void import("@wailsio/runtime")
      .then(({ Events }) => {
        if (!active) {
          return;
        }
        unsubscribe = Events.On("app-sessions:changed", (event: unknown) => {
          const payload = ((event as { data?: unknown })?.data ?? event) as
            | { siteKey?: string }
            | undefined;
          if (!payload?.siteKey || payload.siteKey === "youtube") {
			videoDetailsCacheRef.current.clear();
            setReloadToken((value) => value + 1);
          }
        });
      })
      .catch(() => {});
    return () => {
      active = false;
      unsubscribe();
    };
  }, []);

  React.useEffect(() => {
		const generation = browseGenerationRef.current + 1;
		browseGenerationRef.current = generation;
		requestedContinuationsRef.current.clear();
		setAppending(false);
		setAppendError("");
    if (!active) {
      return;
    }
    if (
      routeId === "search" &&
      !selectedPlaylist &&
      !submittedQuery.trim()
    ) {
      setLoading(false);
      setError("");
      setPage(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError("");
    setPage(null);
	void browseYouTubeWorkspace(
		createYouTubeWorkspaceBrowseRequest(
				routeId,
				submittedQuery,
				selectedPlaylist?.playlistId,
				{ locale: text.locale },
			),
		)
      .then((nextPage) => {
				if (!cancelled && browseGenerationRef.current === generation) {
          setPage(nextPage);
        }
      })
      .catch((reason) => {
				if (!cancelled && browseGenerationRef.current === generation) {
          setPage(null);
          setError(readErrorMessage(reason, text));
        }
      })
      .finally(() => {
				if (!cancelled && browseGenerationRef.current === generation) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
	}, [
		active,
		reloadToken,
		routeId,
		selectedPlaylist?.playlistId,
		submittedQuery,
			text.locale,
		]);

	const loadNextPage = React.useCallback(() => {
		const continuation = page?.continuation?.trim() || "";
		if (
			!active ||
			loading ||
			appending ||
			!continuation ||
			requestedContinuationsRef.current.has(continuation)
		) {
			return;
		}
		requestedContinuationsRef.current.add(continuation);
		const generation = browseGenerationRef.current;
		setAppending(true);
		setAppendError("");
		void browseYouTubeWorkspace(
			createYouTubeWorkspaceBrowseRequest(
				routeId,
				submittedQuery,
				selectedPlaylist?.playlistId,
				{ continuation, locale: text.locale },
			),
		)
			.then((nextPage) => {
				if (browseGenerationRef.current !== generation) {
					return;
				}
				setPage((current) =>
					appendYouTubeWorkspaceBrowsePage(
						current,
						nextPage,
						continuation,
					),
				);
			})
			.catch((reason) => {
				if (browseGenerationRef.current === generation) {
					requestedContinuationsRef.current.delete(continuation);
					setAppendError(readErrorMessage(reason, text));
				}
			})
			.finally(() => {
				if (browseGenerationRef.current === generation) {
					setAppending(false);
				}
			});
	}, [
		active,
		appending,
		loading,
		page?.continuation,
		routeId,
		selectedPlaylist?.playlistId,
		submittedQuery,
		text,
	]);

  const openVideo = React.useCallback(
    (
      video: YouTubeWorkspaceVideo,
      nextQueue = queue,
	  options: YouTubeVideoOpenOptions = {},
    ) => {
	  const allowInBackground = options.allowInBackground === true;
	  const revealWatch = options.revealWatch !== false;
      const playableQueue = nextQueue.filter(isPlayableYouTubeWorkspaceVideo);
      const nextIndex = playableQueue.findIndex(
        (item) => item.videoId === video.videoId,
      );
	  const request = playRequestRef.current + 1;
	  playRequestRef.current = request;
	  const backendRequestID = createVideoSequence(request);
	  pendingPlayRequestIDsRef.current.add(backendRequestID);
	  backgroundPlayRequestRef.current = allowInBackground
		? { request, backendRequestID }
		: null;
	  const requestRoute = routeRef.current;
	  if (revealWatch) {
		setOpeningVideoId(video.videoId);
	  }
      setError("");
      void playYouTubeWorkspaceVideo(video, backendRequestID, text.locale)
        .then((descriptor) => {
		  const commitRequest = shouldCommitYouTubeVideoOpen(
			playRequestRef.current === request,
			activeRef.current,
			routeRef.current === requestRoute,
			allowInBackground,
		  );
		  if (!commitRequest) {
			if (pendingPlayRequestIDsRef.current.delete(backendRequestID)) {
				void cancelYouTubeWorkspacePlay(backendRequestID).catch(() => {});
			}
			return;
		  }
		  pendingPlayRequestIDsRef.current.delete(backendRequestID);
		  void acceptYouTubeWorkspacePlay(backendRequestID).catch(() => {});
		  stoppedPlaybackSessionIDRef.current = "";
          setQueue(playableQueue);
          setCurrentIndex(nextIndex);
          setPlayback(descriptor);
          setPlayerStatus({
            provider: "youtube",
            sessionId: descriptor.sessionId,
            available: true,
            videoId: descriptor.videoId,
            state: "loading",
            title: descriptor.title,
            artist: descriptor.artist,
            thumbnailUrl: descriptor.thumbnailUrl,
            duration: descriptor.durationSeconds,
            currentTime: 0,
          });
		  if (revealWatch && options.returnToUploader) {
			setUploaderReturnTarget(options.returnToUploader);
			setUploaderTarget(null);
		  }
		  setPrimaryDetail((current) =>
			openYouTubePrimaryWatch(
			  current,
			  playableQueue[nextIndex] ?? video,
			  ),
		  );
		  if (revealWatch) {
			onWatchOpenChange(true);
		  }
        })
		.catch((reason) => {
		  pendingPlayRequestIDsRef.current.delete(backendRequestID);
		  if (playRequestRef.current === request) {
			if (watchOpen) {
			  reportControlError(reason);
			} else {
			  setError(readErrorMessage(reason, text));
			}
		  }
		})
		.finally(() => {
		  pendingPlayRequestIDsRef.current.delete(backendRequestID);
		  if (backgroundPlayRequestRef.current?.request === request) {
			backgroundPlayRequestRef.current = null;
		  }
		  if (playRequestRef.current === request) {
			setOpeningVideoId("");
		  }
		});
    },
    [onWatchOpenChange, queue, reportControlError, text, watchOpen],
  );

  const openQueueIndex = React.useCallback(
    (index: number, options?: YouTubeVideoOpenOptions) => {
      const item = queue[index];
      if (item) {
		openVideo(item, queue, options);
      }
    },
    [openVideo, queue],
  );

	React.useEffect(() => {
		const result = consumeYouTubeExternalCommand(
			externalCommand,
			handledExternalCommandID.current,
			currentIndex,
			queue.length,
		);
		if (result.handledID === handledExternalCommandID.current) {
			return;
		}
		handledExternalCommandID.current = result.handledID;
		if (result.clearPlayback) {
			clearStoppedPlayback();
			return;
		}
		if (result.targetIndex >= 0) {
			const revealWatch = externalCommand?.revealWatch !== false;
			openQueueIndex(result.targetIndex, {
				allowInBackground: !revealWatch,
				revealWatch,
			});
		}
	}, [
		clearStoppedPlayback,
		currentIndex,
		externalCommand,
		openQueueIndex,
		queue.length,
	]);

  const togglePlayback = React.useCallback(() => {
    if (!playback?.sessionId) {
      return;
    }
    const playing = playerStatus.state === "playing";
	if (!playing && !playerSurfaceVisible) {
		const video =
			queue.find((item) => item.videoId === playback.videoId) ??
			playbackDescriptorToVideo(playback, playerStatus);
		setPrimaryDetail((current) => openYouTubePrimaryWatch(current, video));
		onWatchOpenChange(true);
		window.requestAnimationFrame(() => {
			void resumeYouTubeWorkspaceVideo(playback.sessionId).catch(
				reportControlError,
			);
		});
		return;
	}
    void (playing
      ? pauseYouTubeWorkspaceVideo(playback.sessionId)
      : resumeYouTubeWorkspaceVideo(playback.sessionId)
    ).catch(reportControlError);
  }, [
	onWatchOpenChange,
	playback,
	playerStatus,
	playerSurfaceVisible,
	queue,
	reportControlError,
  ]);

  const toggleMute = React.useCallback(() => {
    if (!playback?.sessionId) {
      return;
    }
    const sessionID = playback.sessionId;
    const previousMuted = muted;
    const nextMuted = !muted;
    const request = muteRequestRef.current + 1;
    muteRequestRef.current = request;
    setMuted(nextMuted);
		if (volumeWriteTimerRef.current !== null) {
			window.clearTimeout(volumeWriteTimerRef.current);
			volumeWriteTimerRef.current = null;
		}
		const desired = { sessionId: sessionID, volume, muted: nextMuted };
		desiredVolumeRef.current = desired;
    void setYouTubeWorkspaceVolume(sessionID, volume, nextMuted).catch(
      (reason) => {
        if (
          muteRequestRef.current !== request ||
          playbackSessionRef.current !== sessionID
        ) {
          return;
        }
				if (desiredVolumeRef.current === desired) {
					desiredVolumeRef.current = null;
				}
        setMuted((current) =>
          rollbackYouTubeOptimisticMute(current, nextMuted, previousMuted),
        );
        reportControlError(reason);
      },
    );
  }, [muted, playback?.sessionId, reportControlError, volume]);

	const changeVolume = React.useCallback((nextVolume: number) => {
		if (!playback?.sessionId) {
			return;
		}
		const normalized = Math.max(0, Math.min(1, nextVolume));
		const sessionId = playback.sessionId;
		setVolume(normalized);
		const desired = { sessionId, volume: normalized, muted };
		desiredVolumeRef.current = desired;
		if (volumeWriteTimerRef.current !== null) {
			window.clearTimeout(volumeWriteTimerRef.current);
		}
		volumeWriteTimerRef.current = window.setTimeout(() => {
			volumeWriteTimerRef.current = null;
			void setYouTubeWorkspaceVolume(sessionId, normalized, muted).catch(
				(reason) => {
					if (desiredVolumeRef.current === desired) {
						desiredVolumeRef.current = null;
					}
					reportControlError(reason);
				},
			);
		}, 40);
	}, [muted, playback?.sessionId, reportControlError]);

	React.useEffect(() => () => {
		if (volumeWriteTimerRef.current !== null) {
			window.clearTimeout(volumeWriteTimerRef.current);
		}
	}, []);

	const seekPlayback = React.useCallback((seconds: number) => {
		if (!playback?.sessionId) {
			return;
		}
		void seekYouTubeWorkspaceVideo(playback.sessionId, seconds)
			.catch(reportControlError);
	}, [playback?.sessionId, reportControlError]);

	const runPlayerControl = React.useCallback(
		(command: (sessionId: string) => Promise<void>) => {
			if (!playback?.sessionId) {
				return;
			}
			void command(playback.sessionId).catch(reportControlError);
		},
		[playback?.sessionId, reportControlError],
	);

	const runPlayerSelection = React.useCallback(
		(command: (sessionId: string, value: string) => Promise<void>, value: string) => {
			if (!playback?.sessionId) {
				return;
			}
			void command(playback.sessionId, value).catch(reportControlError);
		},
		[playback?.sessionId, reportControlError],
	);

	const rateVideo = React.useCallback(
		(desired: Exclude<YouTubeVideoRating, "none">) => {
			if (!playback?.videoId) {
				return;
			}
			const sessionID = playback.sessionId;
			const previous =
				videoRating ?? playerStatus.selections?.rating ?? "none";
			const next: YouTubeVideoRating =
				previous === desired ? "none" : desired;
			const request = videoRatingRequestRef.current + 1;
			videoRatingRequestRef.current = request;
			setVideoRating(next);
			void rateYouTubeWorkspaceVideo(playback.videoId, next).catch(
				(reason) => {
					if (
						videoRatingRequestRef.current !== request ||
						playbackSessionRef.current !== sessionID
					) {
						return;
					}
					setVideoRating(previous);
					reportControlError(reason);
				},
			);
		},
		[playback, playerStatus.selections?.rating, reportControlError, videoRating],
	);

  const submitSearch = () => {
    const query = submitYouTubeSearchQuery(queryInput);
    if (query) {
      setSubmittedQuery(query);
    }
  };

	const primaryWatchDetail =
		watchOpen && primaryDetail.kind === "watch" ? primaryDetail : null;
	const watchDetail = uploaderTarget ? null : primaryWatchDetail;
	const watchVideoID = primaryWatchDetail?.video.videoId?.trim() || "";
	React.useEffect(() => {
		videoDetailsCacheRef.current.clear();
	}, [text.locale]);
	React.useEffect(() => {
		setVideoSubscriptionOverride(null);
		setSubscriptionBusy(false);
		setVideoInfoOpen(false);
	}, [watchVideoID]);
	React.useEffect(() => {
		const request = videoDetailsRequestRef.current + 1;
		videoDetailsRequestRef.current = request;
		if (!watchVideoID) {
			setVideoDetails(null);
			return;
		}
		const cached = videoDetailsCacheRef.current.get(watchVideoID);
		if (cached) {
			setVideoDetails(cached);
			return;
		}
		setVideoDetails(null);
		void getYouTubeWorkspaceVideoDetails(watchVideoID, text.locale)
			.then((details) => {
				if (
					videoDetailsRequestRef.current !== request ||
					details.videoId !== watchVideoID
				) {
					return;
				}
				videoDetailsCacheRef.current.set(watchVideoID, details);
				setVideoDetails(details);
			})
			.catch(() => {
				// Playback does not depend on the optional rich metadata request.
			});
	}, [reloadToken, text.locale, watchVideoID]);
	React.useLayoutEffect(() => {
		if (!primaryWatchDetail && browseScrollRef.current) {
			browseScrollRef.current.scrollTop = browseScrollTopRef.current;
		}
	}, [Boolean(primaryWatchDetail)]);
	const webURL =
		watchDetail?.video.webUrl ||
		page?.webUrl ||
		selectedPlaylist?.webUrl ||
		routeWebFallback(routeId, submittedQuery);
	const uploaderChannelID =
		videoDetails?.channelId ||
		primaryWatchDetail?.video.channelId ||
		playback?.channelId ||
		"";
	const subscriptionIdentity = youtubeSubscriptionIdentity(
		uploaderChannelID,
		`${active ? "active" : "inactive"}:${routeId}:${watchVideoID}`,
	);
	videoSubscriptionGateRef.current.activate(subscriptionIdentity);
	React.useEffect(() => {
		setSubscriptionBusy(false);
		setVideoSubscriptionOverride(null);
	}, [subscriptionIdentity]);
	const channelSubscribed =
		videoSubscriptionOverride?.identity === subscriptionIdentity
			? videoSubscriptionOverride.value
			: videoDetails?.isSubscribed ?? false;
	const toggleChannelSubscription = React.useCallback(() => {
		const channelID = uploaderChannelID.trim();
		if (!channelID || subscriptionBusy) {
			return;
		}
		const previous = channelSubscribed;
		const next = !previous;
		const request = videoSubscriptionGateRef.current.begin(subscriptionIdentity, channelID);
		setVideoSubscriptionOverride({ identity: subscriptionIdentity, value: next });
		setSubscriptionBusy(true);
		void setYouTubeWorkspaceChannelSubscription(channelID, next)
			.then(() => {
				if (!videoSubscriptionGateRef.current.canReconcile(request)) return;
				// The server mutation remains authoritative even if navigation makes
				// this request stale for the visible controls. Reconcile every cached
				// detail for the affected channel first, then gate view-specific state.
				for (const [videoID, cached] of videoDetailsCacheRef.current) {
					const reconciled = reconcileYouTubeSubscriptionDetails(cached, channelID, next);
					if (reconciled !== cached) videoDetailsCacheRef.current.set(videoID, reconciled);
				}
				setVideoDetails((current) =>
					reconcileYouTubeSubscriptionDetails(current, channelID, next),
				);
				if (!videoSubscriptionGateRef.current.isCurrent(request)) {
					return;
				}
			})
			.catch((reason) => {
				if (!videoSubscriptionGateRef.current.isCurrent(request)) {
					return;
				}
				setVideoSubscriptionOverride({
					identity: subscriptionIdentity,
					value: previous,
				});
				reportControlError(reason);
			})
			.finally(() => {
				if (videoSubscriptionGateRef.current.isCurrent(request)) {
					setSubscriptionBusy(false);
				}
			});
	}, [
		channelSubscribed,
		reportControlError,
		subscriptionIdentity,
		subscriptionBusy,
		uploaderChannelID,
		watchVideoID,
	]);
	const openUploader = React.useCallback(() => {
		const channelID = uploaderChannelID.trim();
		if (!primaryWatchDetail || !youtubeChannelURL(channelID)) {
			return;
		}
		setVideoInfoOpen(false);
		setUploaderTarget({
			channelId: channelID,
			name:
				videoDetails?.channel ||
				primaryWatchDetail.video.channel ||
				playback?.artist ||
				text.workspace.youtube,
			avatarUrl: videoDetails?.channelAvatarUrl,
			subscribed: channelSubscribed,
			videoId: primaryWatchDetail.video.videoId,
		});
		if (upNextOpen) {
			onToggleUpNext?.();
		}
	}, [
		channelSubscribed,
		onToggleUpNext,
		playback?.artist,
		primaryWatchDetail,
		text.workspace.youtube,
		uploaderChannelID,
		videoDetails?.channel,
		videoDetails?.channelAvatarUrl,
		upNextOpen,
	]);
	const openBrowseUploader = React.useCallback(
		(video: YouTubeWorkspaceVideo) => {
			const channelID = video.channelId?.trim() || "";
			const target = createYouTubeBrowseUploaderTarget(
				video,
				text.workspace.youtube,
				channelID === uploaderChannelID.trim() && channelSubscribed,
			);
			if (!target) {
				return;
			}
			invalidateVideoOpenRequests();
			setVideoInfoOpen(false);
			setUploaderReturnTarget(null);
			setUploaderTarget(target);
			if (upNextOpen) {
				onToggleUpNext?.();
			}
		},
			[
				channelSubscribed,
				invalidateVideoOpenRequests,
				onToggleUpNext,
				text.workspace.youtube,
				uploaderChannelID,
				upNextOpen,
			],
		);
	const closeUploader = React.useCallback(() => {
		invalidateVideoOpenRequests();
		setUploaderTarget(null);
	}, [invalidateVideoOpenRequests]);
	const syncUploaderSubscription = React.useCallback(
		(channelID: string, subscribed: boolean) => {
			if (!uploaderTarget) {
				return;
			}
			const sync = resolveYouTubeUploaderSubscriptionSync(
				uploaderTarget.channelId,
				uploaderChannelID,
				channelID,
			);
			if (!sync.accept) return;
			if (sync.updateWatch) {
				setVideoSubscriptionOverride({
					identity: subscriptionIdentity,
					value: subscribed,
				});
				setVideoDetails((current) =>
					current ? { ...current, isSubscribed: subscribed } : current,
				);
			}
			const cached = videoDetailsCacheRef.current.get(uploaderTarget.videoId);
			if (cached) {
				videoDetailsCacheRef.current.set(uploaderTarget.videoId, {
					...cached,
					isSubscribed: subscribed,
				});
			}
		},
		[subscriptionIdentity, uploaderChannelID, uploaderTarget],
	);
	const pageTitle = selectedPlaylist?.title || workspaceRouteLabel(routeId, text);
	const searchPage = routeId === "search" && !selectedPlaylist;
	const searchLanding = searchPage && !submittedQuery.trim();
  const pageContract = uploaderTarget
    ? defineWorkspacePageContract({
        presentation: "primary",
        recipe: "detail",
        routeLabel: uploaderTarget.name || text.workspace.youtube,
        topBar: "host-owned",
        heading: "host-owned",
        contentLayout: "card-grid",
        footer: "none",
        scroll: "content",
        density: "comfortable",
        immersion: "standard",
      })
    : watchDetail && playback
      ? defineWorkspacePageContract({
          presentation: "primary",
          recipe: "detail",
          routeLabel: watchDetail.video.title,
          topBar: "host-owned",
          heading: "host-owned",
          contentLayout: "canvas",
          footer: "host-owned",
          scroll: "none",
          density: "regular",
          immersion: "edge-to-edge",
        })
      : selectedPlaylist
        ? defineWorkspacePageContract({
            presentation: "primary",
            recipe: "detail",
            routeLabel: pageTitle,
            topBar: "navigation",
            heading: "display",
            contentLayout: "card-grid",
            footer: "none",
            scroll: "content",
            density: "comfortable",
            immersion: "standard",
          })
        : searchPage
          ? defineWorkspacePageContract({
              presentation: "primary",
              recipe: "search",
              routeLabel: pageTitle,
              topBar: "search",
              heading: "assistive",
              contentLayout: "card-grid",
              footer: "none",
              scroll: "content",
              density: "comfortable",
              immersion: "standard",
            })
          : defineWorkspacePageContract({
              presentation: "primary",
              recipe: "browse",
              routeLabel: pageTitle,
              topBar: "drag",
              heading: "display",
              contentLayout: "card-grid",
              footer: "none",
              scroll: "content",
              density: "comfortable",
              immersion: "standard",
            });
  const searchLabel = `${text.workspace.search} ${text.workspace.youtube}`;
  const toggleFullscreen = React.useCallback(() => {
	if (!playback?.sessionId) {
		return;
	}
	const signalVersion = fullscreenSignalVersionRef.current;
	setFullscreenRequestPending(true);
	const operation = embeddedFullscreen
		? exitYouTubeEmbeddedVideoFullscreen(playback.sessionId)
		: requestYouTubeEmbeddedVideoFullscreen(playback.sessionId);
	void operation
		.then(() => {
			// fullscreenchange is authoritative when the platform emits it. The
			// successful command is the fallback for engines that only resolve the
			// request without forwarding a change event.
			if (fullscreenSignalVersionRef.current === signalVersion) {
				setEmbeddedFullscreen(!embeddedFullscreen);
			}
		})
		.catch(reportControlError)
		.finally(() => {
			if (fullscreenSignalVersionRef.current === signalVersion) {
				setFullscreenRequestPending(false);
			}
		});
	}, [embeddedFullscreen, playback?.sessionId, reportControlError]);
  const playerNode = playback ? (
    <YouTubeWorkspacePlayer
	  active={active && Boolean(watchDetail) && !videoInfoOpen}
	  geometrySuspended={embeddedFullscreen || fullscreenRequestPending}
      playback={playback}
      status={playerStatus}
      text={text}
    />
  ) : null;
  const transportNode = playbackState && watchDetail ? (
    <YouTubeWorkspaceTransportBar
      playback={playbackState}
	  upNextOpen={upNextOpen}
      labels={{
        player: text.listen.nowPlaying,
        previous: text.listen.previous,
        play: text.listen.play,
        pause: text.listen.pause,
        next: text.listen.next,
        fullscreen: text.completed.previewEnterFullscreen,
		exitFullscreen: text.completed.previewExitFullscreen,
        captions: text.dialogs.subtitles,
        audioTrack: text.dialogs.audioTrack,
        quality: text.dialogs.quality,
        danmaku: text.dialogs.danmaku,
        playbackSpeed: text.youtube.playbackSpeed,
        volume: text.listen.volume,
		mute: text.listen.mute,
		unmute: text.listen.unmute,
		download: text.actions.download,
		upNext: text.listen.upNext,
        unavailable: text.youtube.errors.controlUnavailable,
		off: text.settings.equalizer.status.off,
      }}
      onPrevious={() => openQueueIndex(currentIndex - 1)}
      onTogglePlayback={togglePlayback}
      onNext={() => openQueueIndex(currentIndex + 1)}
	  onDownload={onDownload
		? () => onDownload(playbackState.descriptor.webUrl)
		: undefined}
	  onToggleUpNext={onToggleUpNext}
	  fullscreenActive={embeddedFullscreen}
	  fullscreenButtonRef={fullscreenButtonRef}
      onFullscreen={toggleFullscreen}
      onToggleMute={toggleMute}
	  onToggleCaptions={() => runPlayerControl(toggleYouTubeWorkspaceCaptions)}
	  onSelectCaption={(value) =>
		runPlayerSelection(selectYouTubeWorkspaceCaption, value)
	  }
	  onSelectAudioTrack={(value) =>
		runPlayerSelection(selectYouTubeWorkspaceAudioTrack, value)
	  }
	  onSelectQuality={(value) =>
		runPlayerSelection(selectYouTubeWorkspaceQuality, value)
	  }
	  onSelectPlaybackRate={(value) =>
		runPlayerSelection(selectYouTubeWorkspacePlaybackRate, value)
	  }
		  onVolumeChange={changeVolume}
		  onSeek={seekPlayback}
	    />
  ) : null;
	const upNextPortal =
		watchDetail && playback && upNextOpen && upNextPortalTarget
			? createPortal(
				<YouTubeUpNextCompanion
					queue={queue}
					currentIndex={currentIndex}
					currentVideoId={playback.videoId}
					openingVideoId={openingVideoId}
					locale={text.locale}
					title={text.listen.upNext}
					emptyLabel={text.listen.upNextEmpty}
					fallbackChannel={text.workspace.youtube}
					onOpenVideo={(video) => openVideo(video, queue)}
				/>,
				upNextPortalTarget,
			)
			: null;
	const controlErrorNode = controlError ? (
		<StatusBadge
			className="youtube-workspace-control-error"
			icon={<AlertCircle aria-hidden="true" />}
			tone="danger"
			role="alert"
			aria-live="assertive"
		>
			{controlError}
		</StatusBadge>
	) : null;

	return (
		<WorkspacePage
			contract={pageContract}
			className="youtube-workspace-page app-workspace-primary-subpane relative"
			data-active={active}
			data-reserve-window-controls={reserveWindowControls ? "true" : "false"}
			data-youtube-primary-view={
				uploaderTarget ? "uploader" : watchDetail ? "watch" : "browse"
			}
		>
			{upNextPortal}
			{controlErrorNode}
			{uploaderTarget ? (
				<WorkspacePageContent className="youtube-workspace-scroll youtube-workspace-uploader-scroll">
					<YouTubeUploaderPage
						channelId={uploaderTarget.channelId}
						locale={text.locale}
						fallbackName={uploaderTarget.name}
						fallbackAvatarUrl={uploaderTarget.avatarUrl}
						fallbackSubscribed={uploaderTarget.subscribed}
						labels={{
							back: text.actions.back,
							subscribe: text.listen.artistSubscribe,
							unsubscribe: text.listen.artistUnsubscribe,
							videos: text.listen.video,
							loading: text.youtube.loading,
							empty: text.youtube.empty,
							error: text.youtube.errors.unavailable,
							retry: text.listen.retry,
							loadMore: text.listen.loadMore,
							fallbackChannel: text.workspace.youtube,
							more: text.listen.more,
							description: text.listen.artistBiography,
							close: text.actions.close,
						}}
						onBack={closeUploader}
						onOpenVideo={(video, uploaderQueue) => {
							openVideo(video, uploaderQueue, {
								returnToUploader: uploaderTarget,
							});
						}}
						onSubscriptionChange={syncUploaderSubscription}
					/>
				</WorkspacePageContent>
			) : watchDetail && playback ? (
				<WorkspacePageContent>
				  <YouTubePrimaryWatchPage
					video={watchDetail.video}
					videoDetails={videoDetails}
					playback={playback}
					status={playerStatus}
					rating={videoRating ?? playerStatus.selections?.rating}
					subscribed={channelSubscribed}
					subscriptionBusy={subscriptionBusy}
					infoOpen={videoInfoOpen}
					player={playerNode}
					transport={transportNode}
					locale={text.locale}
					fallbackChannel={text.workspace.youtube}
					labels={{
						back: text.actions.back,
						uploader: text.completed.taskDataFields.uploader,
						more: text.listen.more,
						like: text.listen.youtubeLike,
						dislike: text.listen.youtubeDislike,
						subscribe: text.listen.artistSubscribe,
						unsubscribe: text.listen.artistUnsubscribe,
						download: text.actions.download,
						openURL: text.listen.openPage,
						unavailable: text.youtube.errors.controlUnavailable,
						videoInfo: text.youtube.videoInfo,
						description: text.youtube.description,
						descriptionUnavailable:
							text.youtube.descriptionUnavailable,
						published: text.youtube.published,
						views: text.youtube.views,
						likes: text.youtube.likes,
						close: text.actions.close,
					}}
					onBack={dismissWatch}
					onDownload={onDownload
						? () => onDownload(playback.webUrl)
						: undefined}
					onOpenURL={() => void openExternalURL(playback.webUrl)}
					onLike={() => rateVideo("like")}
					onDislike={() => rateVideo("dislike")}
					onToggleSubscription={uploaderChannelID
						? toggleChannelSubscription
						: undefined}
					onInfoOpenChange={setVideoInfoOpen}
					onOpenUploader={uploaderChannelID ? openUploader : undefined}
				  />
				</WorkspacePageContent>
			) : (
			  <>
				<WorkspacePageTopBar
					className={
						searchPage
							? "youtube-workspace-page-header youtube-workspace-page-header--search"
							: "youtube-workspace-page-header"
					}
					data-youtube-page-header={selectedPlaylist ? "playlist" : routeId}
					reserveWindowControls={reserveWindowControls}
				>
					{selectedPlaylist ? (
						<WorkspacePrimaryHeaderAction
							label={text.actions.back}
							onClick={() =>
								setPrimaryDetail((current) =>
									backFromYouTubePrimaryDetail(current),
								)
							}
						>
							<ArrowLeft className="h-4 w-4" />
						</WorkspacePrimaryHeaderAction>
					) : null}
				</WorkspacePageTopBar>
			  <WorkspacePageContent
				ref={browseScrollRef}
				className={
					searchPage
						? "youtube-workspace-scroll youtube-workspace-scroll--search"
						: "youtube-workspace-scroll"
				}
					onScroll={(event) => {
						browseScrollTopRef.current = event.currentTarget.scrollTop;
					}}
				>
					{searchPage ? (
						<WorkspaceSearchControl
							clearLabel={text.actions.clear}
							onSubmit={submitSearch}
							onValueChange={(nextInput) => {
								setQueryInput(nextInput);
								setSubmittedQuery((current) =>
									updateYouTubeSubmittedQueryOnInput(current, nextInput),
								);
							}}
							placeholder={searchLabel}
							submitLabel={text.workspace.search}
							value={queryInput}
						/>
					) : null}

					{searchLanding ? null : error ? (
          <YouTubeWorkspaceNotice
            icon={<AlertCircle />}
            title={text.youtube.errors.unavailableTitle}
            detail={error}
            actionLabel={text.listen.retry}
            onAction={() => setReloadToken((value) => value + 1)}
            secondaryLabel={text.listen.openPage}
            onSecondary={() => void openExternalURL(webURL)}
          />
        ) : loading && !page ? (
          <YouTubeWorkspaceLoading label={text.youtube.loading} />
        ) : page?.requiresAuthentication ? (
          <YouTubeWorkspaceNotice
            icon={<LogIn />}
            title={text.youtube.signInTitle}
            detail={text.youtube.signInRequired}
            actionLabel={text.listen.openConnections}
            onAction={() => void openYouTubeAppSession(webURL)}
            secondaryLabel={text.listen.refresh}
            onSecondary={() => setReloadToken((value) => value + 1)}
          />
        ) : page?.emptyReason === "search_query_required" ? (
          null
        ) : page && page.items.length === 0 && !page.continuation ? (
          <YouTubeWorkspaceNotice
            icon={<Youtube />}
            title={text.youtube.emptyTitle}
            detail={text.youtube.empty}
            actionLabel={text.listen.openPage}
            onAction={() => void openExternalURL(webURL)}
          />
        ) : (
			  <div
				className="youtube-workspace-grid"
				aria-busy={loading || appending}
			  >
            {page?.items.map((video) => (
              <YouTubeVideoCard
                key={video.videoId || video.playlistId}
                video={video}
                opening={
                  openingVideoId === (video.videoId || video.playlistId)
                }
                selected={
                  Boolean(video.videoId) && playback?.videoId === video.videoId
                }
                fallbackChannel={text.workspace.youtube}
				locale={text.locale}
                onOpen={() => {
				  if (video.itemKind === "playlist") {
					if (video.playlistId) {
					  setPrimaryDetail((current) =>
						openYouTubePrimaryPlaylist(current, {
						  playlistId: video.playlistId || "",
						  title: video.title,
						  webUrl: video.webUrl,
						}),
					  );
					}
                    return;
                  }
				  openVideo(video, page.items);
                }}
				onOpenUploader={youtubeChannelURL(video.channelId)
				  ? () => openBrowseUploader(video)
				  : undefined}
				  />
				))}
				{page?.continuation && !appendError ? (
				  <ListenInfiniteScrollSentinel
					continuation={page.continuation}
					enabled={active && !loading}
					loading={appending}
					onLoadMore={loadNextPage}
					className="col-span-full"
				  />
				) : null}
				{appending ? (
				  <div
					className="youtube-workspace-pagination-state col-span-full flex items-center justify-center gap-2 py-3"
					aria-label={text.youtube.loading}
				  >
					<LoaderCircle className="h-4 w-4 app-motion-spin" />
					<span>{text.youtube.loading}</span>
				  </div>
				) : appendError ? (
				  <div className="youtube-workspace-pagination-error col-span-full flex items-center justify-center gap-2 py-3">
					<AlertCircle className="h-4 w-4" />
					<span>{appendError}</span>
					<Button
					  type="button"
					  size="sm"
					  variant="ghost"
					  className="youtube-workspace-pagination-retry h-7 px-2"
					  onClick={() => setAppendError("")}
					>
					  {text.listen.retry}
					</Button>
				  </div>
				) : null}
			  </div>
        )}
	  </WorkspacePageContent>
	  </>
	  )}
    </WorkspacePage>
  );
}

export interface YouTubePrimaryWatchLabels
	extends Omit<YouTubeVideoInfoLabels, "title"> {
	back: string;
	uploader: string;
	more: string;
	like: string;
	dislike: string;
	subscribe: string;
	unsubscribe: string;
	download: string;
	openURL: string;
	unavailable: string;
	videoInfo: string;
}

export function YouTubePrimaryWatchPage({
	video,
	videoDetails,
	playback,
	status,
	rating,
	subscribed,
	subscriptionBusy,
	infoOpen,
	player,
	transport,
	locale,
	fallbackChannel,
	labels,
	onBack,
	onDownload,
	onOpenURL,
	onLike,
	onDislike,
	onToggleSubscription,
	onInfoOpenChange,
	onOpenUploader,
}: {
	video: YouTubeWorkspaceVideo;
	videoDetails?: YouTubeVideoDetails | null;
	playback: YouTubePlaybackDescriptor;
	status: YouTubePlayerStatus;
	rating?: YouTubeVideoRating;
	subscribed: boolean;
	subscriptionBusy: boolean;
	infoOpen: boolean;
	player: React.ReactNode;
	transport: React.ReactNode;
	locale: string;
	fallbackChannel: string;
	labels: YouTubePrimaryWatchLabels;
	onBack: () => void;
	onDownload?: () => void;
	onOpenURL: () => void;
	onLike: () => void;
	onDislike: () => void;
	onToggleSubscription?: () => void;
	onInfoOpenChange: (open: boolean) => void;
	onOpenUploader?: () => void;
}) {
	const title = resolveYouTubePreferredPlaybackTitle(
		video.videoId,
		video.title,
		playback.title,
		status.title,
		videoDetails?.title,
	);
	const channel =
		videoDetails?.channel ||
		status.artist ||
		playback.artist ||
		video.channel ||
		fallbackChannel;
	const viewCount =
		videoDetails?.viewCount || playback.viewCount || video.viewCount || 0;
	const likeCount = videoDetails?.likeCount || 0;
	const published =
		videoDetails?.publishedLabel ||
		videoDetails?.publishedDate ||
		playback.publishedLabel ||
		video.publishedLabel ||
		"";
	const publishedDisplay = formatYouTubePublishedLabel(published, locale);
	const backdrop =
		videoDetails?.thumbnailUrl ||
		status.thumbnailUrl ||
		playback.thumbnailUrl ||
		video.thumbnailUrl ||
		"";
	const channelAvatarUrl = videoDetails?.channelAvatarUrl?.trim() || "";
	const resolvedDetails: YouTubeVideoDetails = {
		videoId: video.videoId,
		title,
		channel,
		channelId:
			videoDetails?.channelId || playback.channelId || video.channelId,
		channelAvatarUrl,
		thumbnailUrl: backdrop,
		durationSeconds:
			videoDetails?.durationSeconds ||
			status.duration ||
			playback.durationSeconds ||
			video.durationSeconds,
		viewCount,
		likeCount,
		publishedDate: videoDetails?.publishedDate,
		publishedLabel: published,
		description: videoDetails?.description,
		isSubscribed: subscribed,
		webUrl: videoDetails?.webUrl || playback.webUrl || video.webUrl,
	};
	const currentRating = rating ?? status.selections?.rating ?? "none";
	const uploaderIdentity = (
		<>
			<span className="youtube-workspace-watch-uploader-avatar" aria-hidden="true">
				<YouTubeImage
					source={channelAvatarUrl}
					alt=""
					fallback={<UserRound />}
				/>
			</span>
			<strong>{channel}</strong>
		</>
	);

	return (
		<div className="youtube-workspace-watch-page">
			<header className="youtube-workspace-watch-header wails-drag">
				<Button
					type="button"
					variant="ghost"
					size="icon"
					shape="circle"
					className="youtube-workspace-watch-back wails-no-drag"
					aria-label={labels.back}
					title={labels.back}
					onClick={onBack}
				>
					<ArrowLeft className="h-4 w-4" />
				</Button>
				<div className="youtube-workspace-watch-info">
					<h1 title={title}>{title}</h1>
					<div className="youtube-workspace-watch-byline">
						<YouTubeSubscriptionIconButton
							subscribed={subscribed}
							busy={subscriptionBusy}
							label={subscribed ? labels.unsubscribe : labels.subscribe}
							className="youtube-workspace-watch-subscribe"
							disabled={!onToggleSubscription}
							onClick={onToggleSubscription}
						/>
						{onOpenUploader ? (
							<button
								type="button"
								className="youtube-workspace-watch-uploader wails-no-drag"
								aria-label={`${labels.uploader}: ${channel}`}
								onClick={onOpenUploader}
							>
								{uploaderIdentity}
							</button>
						) : (
							<span className="youtube-workspace-watch-uploader">
								{uploaderIdentity}
							</span>
						)}
						<span className="youtube-workspace-watch-stats">
							{publishedDisplay ? (
								<span title={labels.published}>
									<CalendarDays aria-hidden="true" />
									{publishedDisplay}
								</span>
							) : null}
							{viewCount > 0 ? (
								<span title={labels.views}>
									<Eye aria-hidden="true" />
									{formatYouTubeViewCount(viewCount, locale)}
								</span>
							) : null}
							{likeCount > 0 ? (
								<span title={labels.likes}>
									<ThumbsUp aria-hidden="true" />
									{formatYouTubeViewCount(likeCount, locale)}
								</span>
							) : null}
						</span>
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button
									type="button"
									variant="ghost"
									size="compactIcon"
									shape="circle"
									className="youtube-workspace-watch-more wails-no-drag"
									aria-label={labels.more}
									title={labels.more}
								>
									<MoreHorizontal />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent
								side="bottom"
								align="center"
								className="youtube-workspace-watch-more-menu"
							>
								<DropdownMenuItem onSelect={() => onInfoOpenChange(true)}>
									<Info className="h-4 w-4 shrink-0" />
									<span>{labels.videoInfo}</span>
								</DropdownMenuItem>
								<DropdownMenuItem disabled={!onDownload} onSelect={onDownload}>
									<Download className="h-4 w-4 shrink-0" />
									<span>{labels.download}</span>
								</DropdownMenuItem>
								<DropdownMenuSeparator />
								<DropdownMenuItem onSelect={onLike}>
									<ThumbsUp
										className="youtube-workspace-watch-rating-icon h-4 w-4 shrink-0"
										data-active={currentRating === "like"}
									/>
									<span>{labels.like}</span>
								</DropdownMenuItem>
								<DropdownMenuItem onSelect={onDislike}>
									<ThumbsDown
										className="youtube-workspace-watch-rating-icon h-4 w-4 shrink-0"
										data-active={currentRating === "dislike"}
									/>
									<span>{labels.dislike}</span>
								</DropdownMenuItem>
								<DropdownMenuSeparator />
								<DropdownMenuItem onSelect={onOpenURL}>
									<ExternalLink className="h-4 w-4 shrink-0" />
									<span>{labels.openURL}</span>
								</DropdownMenuItem>
								<DropdownMenuItem
									disabled={!onOpenUploader}
									onSelect={onOpenUploader}
								>
									<UserRound className="h-4 w-4 shrink-0" />
									<span className="min-w-0 max-w-60 truncate">{channel}</span>
								</DropdownMenuItem>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</div>
			</header>
			<div className="youtube-workspace-watch-video-region">
				<div className="youtube-workspace-watch-player-shell">
					{player ?? (
						<div className="youtube-workspace-watch-player-placeholder">
							<YouTubeImage
								source={backdrop}
								videoId={video.videoId}
								alt=""
								fallback={<Youtube className="youtube-workspace-placeholder-icon h-8 w-8" />}
							/>
						</div>
					)}
				</div>
			</div>
			{transport}
			<YouTubeVideoInfoDialog
				open={infoOpen}
				onOpenChange={onInfoOpenChange}
				details={resolvedDetails}
				fallbackTitle={title}
				fallbackChannel={channel}
				fallbackThumbnail={backdrop}
				locale={locale}
				labels={{
					title: labels.videoInfo,
					description: labels.description,
					descriptionUnavailable: labels.descriptionUnavailable,
					published: labels.published,
					views: labels.views,
					likes: labels.likes,
					close: labels.close,
				}}
			/>
		</div>
	);
}

function YouTubeWorkspacePlayer({
  active,
  geometrySuspended,
  playback,
  status,
  text,
}: {
  active: boolean;
  geometrySuspended: boolean;
  playback: YouTubePlaybackDescriptor;
  status: YouTubePlayerStatus;
  text: ReturnType<typeof getXiaText>;
}) {
  return (
    <div className="youtube-workspace-player-card">
      <YouTubeNativeVideoSurface
        active={active}
		geometrySuspended={geometrySuspended}
        videoId={playback.videoId}
        poster={status.thumbnailUrl || playback.thumbnailUrl}
        loadingLabel={text.listen.loadingStatus}
      />
      {status.state === "error" ? (
		<GlassSurface
		  asChild
		  elevation="floating"
		  shape="control"
		  surfaceRole="status"
		>
		  <p
		    className="youtube-workspace-player-error app-dream-status-message"
		    data-intent="danger"
		    role="alert"
		  >
		    {readYouTubePlaybackErrorMessage(status, text)}
          </p>
		</GlassSurface>
      ) : null}
    </div>
  );
}


export function YouTubeVideoCard({
  video,
  opening,
  selected,
  fallbackChannel,
  locale,
  metadataPrefix,
  thumbnail,
  onOpen,
	onOpenUploader,
}: {
  video: YouTubeWorkspaceVideo;
  opening: boolean;
  selected: boolean;
  fallbackChannel: string;
  locale: string;
  metadataPrefix?: React.ReactNode;
  /** Allows another trusted station to keep its own image security boundary. */
  thumbnail?: React.ReactNode;
  onOpen: () => void;
	onOpenUploader?: () => void;
}) {
	const channel = video.channel || fallbackChannel;
  return (
	<article
      className="youtube-workspace-video-card"
      data-selected={selected}
	  data-opening={opening}
	  aria-busy={opening}
    >
	  <button
		type="button"
		className="youtube-workspace-video-card__open block w-full min-w-0 p-0"
		data-youtube-video-action="open"
		disabled={opening}
		onClick={onOpen}
	  >
      <span className="youtube-workspace-thumbnail">
        {thumbnail ?? (
          <YouTubeImage
            source={video.thumbnailUrl}
            videoId={video.videoId}
            alt=""
            loading="lazy"
            draggable={false}
			fallback={<Youtube className="youtube-workspace-placeholder-icon h-8 w-8" />}
          />
        )}
        {opening ? (
          <GlassSurface
            asChild
            elevation="embedded"
            shape="card"
            surfaceRole="status"
          >
            <span className="youtube-workspace-card-loading" role="status">
              <LoaderCircle className="h-5 w-5 app-motion-spin" />
            </span>
          </GlassSurface>
        ) : null}
        {video.durationLabel ? (
          <span className="youtube-workspace-duration">{video.durationLabel}</span>
        ) : null}
      </span>
      <span className="youtube-workspace-video-title">
        {video.title}
      </span>
	  </button>
	  <span
		className="youtube-workspace-video-card__metadata mt-1 flex items-center gap-1 truncate"
	  >
		{metadataPrefix}
		{onOpenUploader ? (
		  <button
			type="button"
			className="youtube-workspace-video-card__uploader min-w-0 truncate p-0"
			data-youtube-uploader-action="open"
			disabled={opening}
			onClick={(event) => {
			  event.stopPropagation();
			  onOpenUploader();
			}}
		  >
			{channel}
		  </button>
		) : (
		  <span className="truncate">{channel}</span>
		)}
        {video.viewCount ? (
          <span>· {formatYouTubeViewCount(video.viewCount, locale)}</span>
        ) : null}
      </span>
	</article>
  );
}

function YouTubeWorkspaceNotice({
  icon,
  title,
  detail,
  actionLabel,
  onAction,
  secondaryLabel,
  onSecondary,
}: {
  icon: React.ReactNode;
  title: string;
  detail: string;
  actionLabel?: string;
  onAction?: () => void;
  secondaryLabel?: string;
  onSecondary?: () => void;
}) {
	return (
	  <div className="youtube-workspace-notice">
		<span className="youtube-workspace-notice-icon">{icon}</span>
		<h2 className="youtube-workspace-notice__title mt-4">{title}</h2>
		<p className="youtube-workspace-notice__detail mt-1 max-w-lg">{detail}</p>
      {actionLabel || secondaryLabel ? (
        <div className="mt-4 flex items-center gap-2">
          {actionLabel ? <Button onClick={onAction}>{actionLabel}</Button> : null}
          {secondaryLabel ? (
            <Button variant="outline" onClick={onSecondary}>
              {secondaryLabel}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function YouTubeWorkspaceLoading({ label }: { label: string }) {
  return (
    <div className="youtube-workspace-grid" aria-label={label}>
      {Array.from({ length: 8 }, (_, index) => (
        <div key={index} className="youtube-workspace-skeleton">
          <div />
          <span />
          <span />
        </div>
      ))}
    </div>
  );
}

async function openYouTubeAppSession(targetUrl: string) {
  const { Call } = await import("@wailsio/runtime");
  await Call.ByName(
    "xiadown/internal/presentation/wails.AppSessionsHandler.OpenAppSessionSite",
    { id: "site-app-session-youtube", targetUrl },
  );
}

function normalizeWorkspaceRoute(value: string): YouTubeWorkspaceRouteId {
  switch (value) {
    case "search":
    case "subscriptions":
    case "explore":
    case "shorts":
    case "liked-videos":
    case "watch-later":
    case "playlists":
    case "history":
      return value;
    default:
      return "home";
  }
}

function routeWebFallback(routeId: YouTubeWorkspaceRouteId, query: string) {
  switch (routeId) {
    case "search":
      return query
        ? `https://www.youtube.com/results?search_query=${encodeURIComponent(query)}`
        : "https://www.youtube.com/results";
    case "subscriptions":
      return "https://www.youtube.com/feed/subscriptions";
    case "explore":
      return "https://www.youtube.com/feed/explore";
    case "shorts":
      return "https://www.youtube.com/shorts/";
    case "liked-videos":
      return "https://www.youtube.com/playlist?list=LL";
    case "watch-later":
      return "https://www.youtube.com/playlist?list=WL";
    case "playlists":
      return "https://www.youtube.com/feed/playlists";
    case "history":
      return "https://www.youtube.com/feed/history";
    default:
      return "https://www.youtube.com/";
  }
}

function createVideoSequence(request: number) {
  return Date.now() * 1_000 + (request % 1_000);
}

function playbackDescriptorToVideo(
  playback: YouTubePlaybackDescriptor,
  status: YouTubePlayerStatus,
): YouTubeWorkspaceVideo {
  return {
    itemKind: "video",
    videoId: playback.videoId,
    title: resolveYouTubePreferredPlaybackTitle(
		playback.videoId,
		playback.title,
		status.title,
	),
    channel: status.artist || playback.artist,
    channelId: playback.channelId,
    thumbnailUrl: status.thumbnailUrl || playback.thumbnailUrl,
    durationSeconds: status.duration || playback.durationSeconds,
    viewCount: playback.viewCount,
    publishedLabel: playback.publishedLabel,
    webUrl: playback.webUrl,
  };
}

function isPlayableYouTubeWorkspaceVideo(video: YouTubeWorkspaceVideo) {
  return (
    video.itemKind !== "playlist" &&
    /^[A-Za-z0-9_-]{11}$/.test(video.videoId.trim())
  );
}

function workspaceRouteLabel(
  routeId: YouTubeWorkspaceRouteId,
  text: ReturnType<typeof getXiaText>,
) {
  switch (routeId) {
    case "search":
      return text.workspace.search;
    case "subscriptions":
      return text.workspace.subscriptions;
    case "explore":
      return text.workspace.explore;
    case "shorts":
      return text.workspace.shorts;
    case "liked-videos":
      return text.workspace.likedVideos;
    case "watch-later":
      return text.workspace.watchLater;
    case "playlists":
      return text.workspace.playlists;
    case "history":
      return text.workspace.history;
    default:
      return text.workspace.home;
  }
}

function readErrorMessage(
  reason: unknown,
  text: ReturnType<typeof getXiaText>,
  scope: "browse" | "playback" | "control" = "browse",
) {
  return resolveYouTubeWorkspaceErrorMessage(reason, text.youtube.errors, scope);
}

function readYouTubePlaybackErrorMessage(
	status: YouTubePlayerStatus,
	text: ReturnType<typeof getXiaText>,
) {
	const errorCode = String(status.errorCode || "").trim();
	if (errorCode === LISTEN_YOUTUBE_VERIFICATION_REQUIRED_ERROR_CODE) {
		return text.listen.youtubeVerificationRequired;
	}
	if (errorCode === LISTEN_YOUTUBE_REGION_UNAVAILABLE_ERROR_CODE) {
		return text.listen.youtubeRegionUnavailable;
	}
	return (
		String(status.errorMessage || "").trim() ||
		errorCode ||
		text.youtube.errors.playerUnavailable
	);
}
