import { useQueryClient } from "@tanstack/react-query";
import {
  Cloud,
  DownloadCloud,
  Egg,
  Film,
  Globe2,
  Moon,
  Package,
  Volume2,
  VolumeX,
  Waves,
} from "lucide-react";
import * as React from "react";

import { CORE_DEPENDENCIES } from "@/app/main/main-constants";
import {
  getXiaText,
  mergeXiaAppearanceConfig,
  readXiaAppearance,
  XIA_THEME_PACKS,
  type XiaThemePack,
  type XiaThemePackId,
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { Settings } from "@/shared/contracts/settings";
import {
  setLanguage as setI18nLanguage,
  t,
  type SupportedLanguage,
} from "@/shared/i18n";
import {
  useDependencies,
  useDependencyInstallState,
  useDependencyUpdates,
  useInstallDependency,
} from "@/shared/query/dependencies";
import {
  setLatestSettingsQueryData,
  useSystemProxyInfo,
  useUpdateSettings,
} from "@/shared/query/settings";
import type { DependencyInstallState } from "@/shared/contracts/dependencies";

export const WELCOME_DEBUG_EVENT = "xiadown:welcome-debug";
const WELCOME_OCEAN_WAVES_AUDIO_URL = "/audio/welcome-ocean-waves.mp3";

export type WelcomeDebugStep = "proxy" | "dependencies" | "ready";

export type WelcomeDebugCommand =
  | { type: "hide" | "show" }
  | { type: "step"; step: WelcomeDebugStep }
  | { type: "proxy"; mode: WelcomeProxyChoice };

export type WelcomeDebugAPI = {
  hide: () => void;
  show: () => void;
  reset: () => void;
  step: (step: WelcomeDebugStep) => void;
  proxy: (mode: WelcomeProxyChoice) => void;
};

declare global {
  interface Window {
    xiadownWelcome?: WelcomeDebugAPI;
  }
}

type WelcomeProxyChoice = "none" | "system";
type DependencyName = (typeof CORE_DEPENDENCIES)[number];
type DependencyWelcomeStage = "idle" | "installing" | "done" | "error";
type WelcomeMotionPhase =
  | "idle"
  | "proxy-confirm"
  | "proxy-exit"
  | "dependencies-enter"
  | "dependencies-back"
  | "installing"
  | "dependencies-complete"
  | "proxy-return"
  | "ready-enter";
type WelcomeDropdownOption<T extends string> = {
  label: string;
  swatch?: XiaThemePack["preview"];
  value: T;
};

type DependencyWelcomeState = {
  progress: number;
  stage: DependencyWelcomeStage;
};

type WelcomeText = {
  bgmOff: string;
  bgmOn: string;
  enter: string;
  installAll: string;
  installing: string;
  language: string;
  noProxyDescription: string;
  proxyNone: string;
  proxySystem: string;
  systemProxyDescription: string;
  readyHint: string;
  readyTitle: string;
  stageDependencies: string;
  stageProxy: string;
  stageReady: string;
  systemProxyTitle: string;
  theme: string;
  title: string;
};

const DEPENDENCY_META: Record<
  DependencyName,
  {
    Icon: React.ComponentType<React.SVGProps<SVGSVGElement>>;
    displayName: string;
  }
> = {
  "yt-dlp": { Icon: DownloadCloud, displayName: "YT-DLP" },
  ffmpeg: { Icon: Film, displayName: "FFMPEG" },
  bun: { Icon: Package, displayName: "BUN" },
};

function buildWelcomeText(language: "en" | "zh-CN"): WelcomeText {
  return {
    bgmOff: t("xiadown.welcome.bgmOff", language),
    bgmOn: t("xiadown.welcome.bgmOn", language),
    enter: t("xiadown.welcome.enterApp", language),
    installAll: t("xiadown.welcome.installAll", language),
    installing: t("xiadown.welcome.installing", language),
    language: t("xiadown.welcome.language", language),
    noProxyDescription: t("xiadown.welcome.noProxyDescription", language),
    proxyNone: t("xiadown.welcome.proxyNone", language),
    proxySystem: t("xiadown.welcome.proxySystem", language),
    systemProxyDescription: t("xiadown.welcome.systemProxyDescription", language),
    readyHint: t("xiadown.welcome.readyHint", language),
    readyTitle: t("xiadown.welcome.readyTitle", language),
    stageDependencies: t("xiadown.welcome.stageDependencies", language),
    stageProxy: t("xiadown.welcome.stageProxy", language),
    stageReady: t("xiadown.welcome.stageReady", language),
    systemProxyTitle: t("xiadown.welcome.systemProxyTitle", language),
    theme: t("xiadown.welcome.theme", language),
    title: t("xiadown.welcome.title", language),
  };
}

function welcomeNoise(index: number, salt: number) {
  const value = Math.sin(index * 12.9898 + salt * 78.233) * 43758.5453;
  return value - Math.floor(value);
}

const WELCOME_STARS = Array.from({ length: 72 }, (_, index) => {
  const size = welcomeNoise(index, 7) > 0.84 ? 3 : welcomeNoise(index, 13) > 0.48 ? 2 : 1;
  return {
    delay: `${-(welcomeNoise(index, 2) * 5.8).toFixed(2)}s`,
    duration: `${(2.1 + welcomeNoise(index, 3) * 4.7).toFixed(2)}s`,
    left: `${(3 + welcomeNoise(index, 5) * 93).toFixed(2)}%`,
    maxOpacity: (0.58 + welcomeNoise(index, 11) * 0.42).toFixed(2),
    minOpacity: (0.18 + welcomeNoise(index, 17) * 0.34).toFixed(2),
    size: `${size}px`,
    top: `${(4 + Math.pow(welcomeNoise(index, 19), 1.35) * 51).toFixed(2)}%`,
  };
});

const WELCOME_SHOOTING_STARS = [
  {
    delay: "-1.4s",
    duration: "11.6s",
    left: "76%",
    top: "11%",
    travelX: "-290px",
    travelY: "118px",
    width: "84px",
  },
  {
    delay: "-7.8s",
    duration: "16.2s",
    left: "58%",
    top: "24%",
    travelX: "-210px",
    travelY: "74px",
    width: "54px",
  },
  {
    delay: "-12.2s",
    duration: "21.4s",
    left: "91%",
    top: "18%",
    travelX: "-360px",
    travelY: "140px",
    width: "96px",
  },
];

type WelcomePixelRect = readonly [number, number, number, number];

const WELCOME_CHICKEN_PIXEL_LAYERS: Array<{
  className: string;
  fill: string;
  rects: WelcomePixelRect[];
}> = [
  {
    className: "welcome-chicken-leg-a",
    fill: "#b96f2b",
    rects: [
      [17, 35, 2, 7],
      [14, 42, 7, 2],
      [14, 44, 3, 1],
    ],
  },
  {
    className: "welcome-chicken-leg-b",
    fill: "#b96f2b",
    rects: [
      [27, 35, 2, 7],
      [24, 42, 7, 2],
      [28, 44, 3, 1],
    ],
  },
  {
    className: "welcome-chicken-body-outline",
    fill: "#6f3f22",
    rects: [
      [5, 17, 6, 3],
      [3, 20, 7, 3],
      [8, 22, 4, 3],
      [8, 15, 22, 2],
      [6, 17, 29, 3],
      [4, 20, 34, 11],
      [6, 31, 29, 4],
      [10, 35, 21, 3],
      [16, 38, 11, 2],
    ],
  },
  {
    className: "welcome-chicken-body-base",
    fill: "#f7df9a",
    rects: [
      [7, 18, 3, 2],
      [5, 21, 5, 2],
      [9, 23, 3, 2],
      [9, 17, 21, 2],
      [7, 19, 27, 3],
      [6, 22, 30, 8],
      [8, 30, 26, 4],
      [12, 34, 18, 3],
    ],
  },
  {
    className: "welcome-chicken-body-highlight",
    fill: "#fff3bd",
    rects: [
      [10, 18, 13, 2],
      [8, 21, 12, 3],
      [12, 24, 9, 2],
    ],
  },
  {
    className: "welcome-chicken-wing-layer",
    fill: "#d89d45",
    rects: [
      [13, 24, 15, 3],
      [11, 27, 18, 3],
      [14, 30, 13, 2],
    ],
  },
  {
    className: "welcome-chicken-head-layer",
    fill: "#6f3f22",
    rects: [
      [32, 10, 8, 2],
      [30, 12, 12, 3],
      [29, 15, 14, 6],
      [31, 21, 10, 3],
    ],
  },
  {
    className: "welcome-chicken-head-layer",
    fill: "#f8e09a",
    rects: [
      [32, 12, 8, 2],
      [31, 14, 10, 6],
      [33, 20, 7, 2],
    ],
  },
  {
    className: "welcome-chicken-head-layer",
    fill: "#fff4bf",
    rects: [
      [33, 13, 5, 2],
      [32, 15, 4, 2],
    ],
  },
  {
    className: "welcome-chicken-comb-layer",
    fill: "#d94943",
    rects: [
      [34, 5, 2, 4],
      [37, 6, 2, 4],
      [32, 8, 9, 3],
    ],
  },
  {
    className: "welcome-chicken-comb-layer",
    fill: "#ff6658",
    rects: [
      [34, 5, 1, 2],
      [37, 6, 1, 2],
      [33, 8, 4, 1],
    ],
  },
  {
    className: "welcome-chicken-beak-layer",
    fill: "#e79632",
    rects: [
      [41, 16, 5, 2],
      [42, 18, 4, 2],
    ],
  },
  {
    className: "welcome-chicken-eye-layer",
    fill: "#38263f",
    rects: [[36, 15, 2, 2]],
  },
  {
    className: "welcome-chicken-dust",
    fill: "#805332",
    rects: [
      [43, 39, 1, 1],
      [46, 40, 1, 1],
      [41, 42, 1, 1],
    ],
  },
];

function WelcomePixelChicken() {
  return (
    <svg
      className="welcome-chicken-pet"
      viewBox="0 0 48 48"
      aria-hidden="true"
    >
      {WELCOME_CHICKEN_PIXEL_LAYERS.map((layer, layerIndex) => (
        <g key={`${layer.className}-${layerIndex}`} className={layer.className} fill={layer.fill}>
          {layer.rects.map(([x, y, width, height]) => (
            <rect
              key={`${x}-${y}-${width}-${height}`}
              x={x}
              y={y}
              width={width}
              height={height}
            />
          ))}
        </g>
      ))}
    </svg>
  );
}

function createInitialDependencyState(
  installedNames: ReadonlySet<DependencyName> = new Set<DependencyName>(),
): Record<DependencyName, DependencyWelcomeState> {
  return CORE_DEPENDENCIES.reduce(
    (accumulator, name) => {
      accumulator[name] = installedNames.has(name)
        ? { progress: 100, stage: "done" }
        : { progress: 0, stage: "idle" };
      return accumulator;
    },
    {} as Record<DependencyName, DependencyWelcomeState>,
  );
}

function clampPercent(value: number) {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.min(100, Math.max(0, Math.round(value)));
}

function isDependencyName(value: string): value is DependencyName {
  return CORE_DEPENDENCIES.includes(value as DependencyName);
}

function isThemePackId(value: string): value is XiaThemePackId {
  return XIA_THEME_PACKS.some((item) => item.id === value);
}

function normalizeWelcomeLanguage(language: string | undefined): SupportedLanguage {
  return language?.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

function normalizeDependencyInstallStage(stage?: string) {
  return (stage ?? "").trim().toLowerCase();
}

function isWelcomeDependencyInstallActive(stage?: string) {
  switch (normalizeDependencyInstallStage(stage)) {
    case "downloading":
    case "extracting":
    case "verifying":
    case "installing":
      return true;
    default:
      return false;
  }
}

function isDependencyInstallStateFresh(
  installState: DependencyInstallState | undefined,
  startedAt: number | undefined,
) {
  if (!startedAt || !installState?.updatedAt) {
    return true;
  }
  const updatedAt = Date.parse(installState.updatedAt);
  if (!Number.isFinite(updatedAt)) {
    return true;
  }
  return updatedAt >= startedAt - 250;
}

function createWelcomeProxySettings(
  settings: Settings | null,
  mode: WelcomeProxyChoice,
): Settings["proxy"] {
  const current = settings?.proxy;
  return {
    mode,
    scheme: current?.scheme ?? "http",
    host: current?.host ?? "",
    port: current?.port ?? 0,
    username: current?.username ?? "",
    password: current?.password ?? "",
    noProxy: [...(current?.noProxy ?? [])],
    timeoutSeconds: current?.timeoutSeconds ?? 30,
    testedAt: "",
    testSuccess: false,
    testMessage: "",
  };
}

function dependencyProgressStyle(progress: number) {
  const clampedProgress = clampPercent(progress);
  const snappedProgress =
    clampedProgress === 0 ? 0 : Math.ceil(clampedProgress / 6.25) * 6.25;
  return {
    "--dependency-progress": `${Math.min(360, snappedProgress * 3.6)}deg`,
  } as React.CSSProperties;
}

function dependencyLatestVersionLabel(
  version: string | undefined,
  checkingLabel: string,
  fallbackLabel: string,
  loading: boolean,
) {
  const trimmed = version?.trim();
  if (trimmed) {
    return trimmed;
  }
  return loading ? checkingLabel : fallbackLabel;
}

function WelcomeThemeSwatch(props: { swatch: XiaThemePack["preview"] }) {
  return (
    <span
      className="welcome-theme-swatch"
      style={
        {
          "--welcome-swatch-accent": props.swatch.accent,
          "--welcome-swatch-shell": props.swatch.shell,
          "--welcome-swatch-sidebar": props.swatch.sidebar,
        } as React.CSSProperties
      }
      aria-hidden="true"
    />
  );
}

function WelcomeDropdown<T extends string>(props: {
  disabled?: boolean;
  label: string;
  onChange: (value: T) => void | Promise<void>;
  options: Array<WelcomeDropdownOption<T>>;
  value: T;
}) {
  const [open, setOpen] = React.useState(false);
  const dropdownId = React.useId();
  const rootRef = React.useRef<HTMLDivElement | null>(null);
  const selectedOption =
    props.options.find((option) => option.value === props.value) ??
    props.options[0];

  React.useEffect(() => {
    if (!open) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  React.useEffect(() => {
    if (props.disabled) {
      setOpen(false);
    }
  }, [props.disabled]);

  return (
    <div className="welcome-dropdown" ref={rootRef}>
      <span id={`${dropdownId}-label`} className="welcome-dropdown-label">
        {props.label}
      </span>
      <button
        type="button"
        className="welcome-dropdown-trigger"
        aria-controls={`${dropdownId}-menu`}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-labelledby={`${dropdownId}-label ${dropdownId}-value`}
        disabled={props.disabled}
        onClick={() => setOpen((current) => !current)}
      >
        {selectedOption?.swatch ? (
          <WelcomeThemeSwatch swatch={selectedOption.swatch} />
        ) : null}
        <span id={`${dropdownId}-value`} className="welcome-dropdown-value">
          {selectedOption?.label ?? props.value}
        </span>
        <span className="welcome-dropdown-icon" aria-hidden="true">
          {open ? "▲" : "▼"}
        </span>
      </button>
      {open ? (
        <div
          id={`${dropdownId}-menu`}
          className="welcome-dropdown-menu"
          role="listbox"
          aria-labelledby={`${dropdownId}-label`}
        >
          {props.options.map((option) => {
            const selected = option.value === props.value;
            return (
              <button
                key={option.value}
                type="button"
                className={cn("welcome-dropdown-item", selected && "is-selected")}
                role="option"
                aria-selected={selected}
                onClick={() => {
                  setOpen(false);
                  void props.onChange(option.value);
                }}
              >
                {option.swatch ? <WelcomeThemeSwatch swatch={option.swatch} /> : null}
                <span className="welcome-dropdown-item-label">{option.label}</span>
                <span className="welcome-dropdown-check" aria-hidden="true">
                  {selected ? ">" : ""}
                </span>
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

function useOceanWaveBgm() {
  const audioRef = React.useRef<HTMLAudioElement | null>(null);
  const [playing, setPlaying] = React.useState(false);

  const stop = React.useCallback(() => {
    const audio = audioRef.current;
    audioRef.current = null;
    if (!audio) {
      setPlaying(false);
      return;
    }
    audio.pause();
    audio.removeAttribute("src");
    audio.load();
    setPlaying(false);
  }, []);

  const start = React.useCallback(() => {
    if (audioRef.current) {
      return;
    }
    const audio = new Audio(WELCOME_OCEAN_WAVES_AUDIO_URL);
    audio.loop = true;
    audio.volume = 0.72;
    audio.preload = "auto";
    audioRef.current = audio;
    audio
      .play()
      .then(() => {
        setPlaying(true);
      })
      .catch(() => {
        if (audioRef.current === audio) {
          audioRef.current = null;
        }
        audio.pause();
        setPlaying(false);
      });
  }, []);

  React.useEffect(() => stop, [stop]);

  return {
    playing,
    toggle: playing ? stop : start,
  };
}

export function WelcomeScreen(props: {
  open: boolean;
  settings: Settings | null;
  onComplete: () => void;
}) {
  const queryClient = useQueryClient();
  const updateSettings = useUpdateSettings();
  const dependenciesQuery = useDependencies({
    refetchInterval: props.open ? 3_000 : false,
  });
  const dependencyUpdatesQuery = useDependencyUpdates();
  const installDependency = useInstallDependency();
  const ytdlpInstallState = useDependencyInstallState("yt-dlp", props.open);
  const ffmpegInstallState = useDependencyInstallState("ffmpeg", props.open);
  const bunInstallState = useDependencyInstallState("bun", props.open);
  const [step, setStep] = React.useState<WelcomeDebugStep>("proxy");
  const [proxyChoice, setProxyChoice] = React.useState<WelcomeProxyChoice>("none");
  const systemProxyQuery = useSystemProxyInfo(
    props.open && proxyChoice === "system",
  );
  const [language, setLanguageState] = React.useState<SupportedLanguage>(
    normalizeWelcomeLanguage(props.settings?.language),
  );
  const [themePackId, setThemePackId] = React.useState<XiaThemePackId>(() =>
    readXiaAppearance(props.settings).themePackId,
  );
  const [dependencyState, setDependencyState] = React.useState<
    Record<DependencyName, DependencyWelcomeState>
  >(() => createInitialDependencyState());
  const [motionPhase, setMotionPhase] = React.useState<WelcomeMotionPhase>("idle");
  const installRunIdRef = React.useRef(0);
  const dependencyInstallStartedAtRef = React.useRef(
    new Map<DependencyName, number>(),
  );
  const motionRunIdRef = React.useRef(0);
  const dependencyReadyTimerRef = React.useRef<number | null>(null);
  const dependencyIdleTimerRef = React.useRef<number | null>(null);
  const bgm = useOceanWaveBgm();
  const text = getXiaText(language);
  const welcomeText = React.useMemo(
    () => buildWelcomeText(text.locale),
    [text.locale],
  );
  const systemProxyAddress = (systemProxyQuery.data?.address ?? "").trim();
  const systemProxyDisplay = systemProxyAddress
    ? systemProxyAddress
    : systemProxyQuery.isLoading || systemProxyQuery.isFetching
      ? text.settings.checking
      : systemProxyQuery.isError
        ? text.settings.unavailable
        : text.settings.systemProxyEmpty;
  const installedDependencyNames = React.useMemo(() => {
    const names = new Set<DependencyName>();
    for (const dependency of dependenciesQuery.data ?? []) {
      if (!isDependencyName(dependency.name)) {
        continue;
      }
      if ((dependency.status ?? "").trim().toLowerCase() === "installed") {
        names.add(dependency.name);
      }
    }
    return names;
  }, [dependenciesQuery.data]);
  const dependencyUpdatesByName = React.useMemo(
    () =>
      new Map(
        (dependencyUpdatesQuery.data ?? []).map((item) => [item.name, item]),
      ),
    [dependencyUpdatesQuery.data],
  );
  const installStatesByName = React.useMemo(
    () =>
      new Map<DependencyName, DependencyInstallState | undefined>([
        ["yt-dlp", ytdlpInstallState.data],
        ["ffmpeg", ffmpegInstallState.data],
        ["bun", bunInstallState.data],
      ]),
    [bunInstallState.data, ffmpegInstallState.data, ytdlpInstallState.data],
  );
  const allDependenciesDone = CORE_DEPENDENCIES.every(
    (name) =>
      installedDependencyNames.has(name) || dependencyState[name].stage === "done",
  );
  const dependenciesInstalling =
    motionPhase === "installing" ||
    installDependency.isPending ||
    CORE_DEPENDENCIES.some(
      (name) => dependencyState[name].stage === "installing",
    );
  const proxyLocked =
    motionPhase === "proxy-confirm" ||
    motionPhase === "proxy-exit" ||
    motionPhase === "proxy-return";
  const dependencyNavigationLocked =
    motionPhase === "dependencies-enter" ||
    motionPhase === "dependencies-back" ||
    motionPhase === "installing" ||
    motionPhase === "dependencies-complete" ||
    motionPhase === "ready-enter";

  const clearDependencyTransitionTimers = React.useCallback(() => {
    if (dependencyReadyTimerRef.current !== null) {
      window.clearTimeout(dependencyReadyTimerRef.current);
      dependencyReadyTimerRef.current = null;
    }
    if (dependencyIdleTimerRef.current !== null) {
      window.clearTimeout(dependencyIdleTimerRef.current);
      dependencyIdleTimerRef.current = null;
    }
  }, []);

  React.useEffect(() => clearDependencyTransitionTimers, [clearDependencyTransitionTimers]);

  React.useEffect(() => {
    if (!props.open) {
      return;
    }
    setLanguageState(normalizeWelcomeLanguage(props.settings?.language));
    setThemePackId(readXiaAppearance(props.settings).themePackId);
  }, [props.open, props.settings]);

  React.useEffect(() => {
    if (!props.open || installedDependencyNames.size === 0) {
      return;
    }
    setDependencyState((current) => {
      let changed = false;
      const next = { ...current };
      for (const name of installedDependencyNames) {
        if (next[name]?.stage === "done") {
          continue;
        }
        next[name] = { progress: 100, stage: "done" };
        changed = true;
      }
      return changed ? next : current;
    });
  }, [installedDependencyNames, props.open]);

  const resetDependencies = React.useCallback(() => {
    installRunIdRef.current += 1;
    dependencyInstallStartedAtRef.current.clear();
    motionRunIdRef.current += 1;
    clearDependencyTransitionTimers();
    setDependencyState(createInitialDependencyState(installedDependencyNames));
    setMotionPhase("idle");
  }, [clearDependencyTransitionTimers, installedDependencyNames]);

  const enterReadyStep = React.useCallback(() => {
    installRunIdRef.current += 1;
    dependencyInstallStartedAtRef.current.clear();
    motionRunIdRef.current += 1;
    clearDependencyTransitionTimers();
    setStep("ready");
    setMotionPhase("ready-enter");
    window.setTimeout(() => {
      setMotionPhase((current) => (current === "ready-enter" ? "idle" : current));
    }, 520);
  }, [clearDependencyTransitionTimers]);

  const transitionDependenciesToReady = React.useCallback(() => {
    motionRunIdRef.current += 1;
    const motionRunId = motionRunIdRef.current;
    clearDependencyTransitionTimers();
    setMotionPhase("dependencies-complete");
    dependencyReadyTimerRef.current = window.setTimeout(() => {
      dependencyReadyTimerRef.current = null;
      if (motionRunIdRef.current !== motionRunId) {
        return;
      }
      setStep("ready");
      setMotionPhase("ready-enter");
    }, 760);
    dependencyIdleTimerRef.current = window.setTimeout(() => {
      dependencyIdleTimerRef.current = null;
      if (motionRunIdRef.current !== motionRunId) {
        return;
      }
      setMotionPhase("idle");
    }, 1280);
  }, [clearDependencyTransitionTimers]);

  React.useEffect(() => {
    if (!props.open) {
      return;
    }
    setDependencyState((current) => {
      let changed = false;
      const next = { ...current };
      for (const name of CORE_DEPENDENCIES) {
        if (installedDependencyNames.has(name)) {
          if (next[name]?.stage !== "done" || next[name]?.progress !== 100) {
            next[name] = { progress: 100, stage: "done" };
            changed = true;
          }
          continue;
        }

        const installState = installStatesByName.get(name);
        const installStage = normalizeDependencyInstallStage(installState?.stage);
        if (!installStage || installStage === "idle") {
          continue;
        }
        if (
          !isDependencyInstallStateFresh(
            installState,
            dependencyInstallStartedAtRef.current.get(name),
          )
        ) {
          continue;
        }

        const currentState = next[name] ?? { progress: 0, stage: "idle" };
        const progress = clampPercent(installState?.progress ?? currentState.progress);
        let resolved: DependencyWelcomeState | null = null;
        if (isWelcomeDependencyInstallActive(installStage)) {
          resolved = {
            progress: Math.max(progress, currentState.progress, 1),
            stage: "installing",
          };
        } else if (installStage === "done") {
          resolved = { progress: 100, stage: "done" };
        } else if (installStage === "error") {
          resolved = {
            progress: Math.max(progress, currentState.progress),
            stage: "error",
          };
        }
        if (
          resolved &&
          (resolved.stage !== currentState.stage ||
            resolved.progress !== currentState.progress)
        ) {
          next[name] = resolved;
          changed = true;
        }
      }
      return changed ? next : current;
    });
  }, [installStatesByName, installedDependencyNames, props.open]);

  React.useEffect(() => {
    if (!props.open || motionPhase !== "installing") {
      return;
    }
    const hasInstallError = CORE_DEPENDENCIES.some(
      (name) => dependencyState[name].stage === "error",
    );
    if (hasInstallError) {
      setMotionPhase("idle");
    }
  }, [dependencyState, motionPhase, props.open]);

  React.useEffect(() => {
    if (!props.open || motionPhase !== "installing" || !allDependenciesDone) {
      return;
    }
    void dependenciesQuery.refetch();
    transitionDependenciesToReady();
  }, [allDependenciesDone, dependenciesQuery, motionPhase, props.open, transitionDependenciesToReady]);

  React.useEffect(() => {
    const handleDebugEvent = (event: Event) => {
      const detail = (event as CustomEvent<WelcomeDebugCommand>).detail;
      if (!detail || typeof detail !== "object") {
        return;
      }
      if (detail.type === "step") {
        motionRunIdRef.current += 1;
        setMotionPhase("idle");
        setStep(detail.step);
        if (detail.step === "dependencies") {
          resetDependencies();
        }
        if (detail.step === "ready") {
          enterReadyStep();
        }
        return;
      }
      if (detail.type === "proxy") {
        motionRunIdRef.current += 1;
        setMotionPhase("idle");
        setProxyChoice(detail.mode);
        setStep("proxy");
      }
    };

    window.addEventListener(WELCOME_DEBUG_EVENT, handleDebugEvent);
    return () => {
      window.removeEventListener(WELCOME_DEBUG_EVENT, handleDebugEvent);
    };
  }, [enterReadyStep, resetDependencies]);

  const handleLanguageChange = async (nextLanguage: SupportedLanguage) => {
    const fallbackSettings = props.settings;
    const fallbackLanguage = normalizeWelcomeLanguage(fallbackSettings?.language);

    setLanguageState(nextLanguage);
    setI18nLanguage(nextLanguage);
    if (fallbackSettings) {
      setLatestSettingsQueryData(queryClient, {
        ...fallbackSettings,
        language: nextLanguage,
      });
    }

    try {
      await updateSettings.mutateAsync({ language: nextLanguage });
    } catch {
      setLanguageState(fallbackLanguage);
      setI18nLanguage(fallbackLanguage);
      if (fallbackSettings) {
        setLatestSettingsQueryData(queryClient, fallbackSettings);
      }
    }
  };

  const handleThemeChange = async (nextThemePackId: XiaThemePackId) => {
    if (!isThemePackId(nextThemePackId)) {
      return;
    }
    const fallbackThemePackId = themePackId;
    const fallbackSettings = props.settings;
    const nextAppearanceConfig = mergeXiaAppearanceConfig(props.settings, {
      themePackId: nextThemePackId,
    });
    setThemePackId(nextThemePackId);
    if (fallbackSettings) {
      setLatestSettingsQueryData(queryClient, {
        ...fallbackSettings,
        appearanceConfig: nextAppearanceConfig,
      });
    }

    try {
      await updateSettings.mutateAsync({
        appearanceConfig: nextAppearanceConfig,
      });
    } catch {
      setThemePackId(fallbackThemePackId);
      if (fallbackSettings) {
        setLatestSettingsQueryData(queryClient, fallbackSettings);
      }
    }
  };

  const persistWelcomeProxyChoice = React.useCallback(async () => {
    const fallbackSettings = props.settings;
    const proxy = createWelcomeProxySettings(fallbackSettings, proxyChoice);
    if (fallbackSettings) {
      setLatestSettingsQueryData(queryClient, {
        ...fallbackSettings,
        proxy,
      });
    }

    try {
      await updateSettings.mutateAsync({ proxy });
    } catch {
      if (fallbackSettings) {
        setLatestSettingsQueryData(queryClient, fallbackSettings);
      }
    }
  }, [props.settings, proxyChoice, queryClient, updateSettings]);

  const handleProxyNext = () => {
    if (proxyLocked) {
      return;
    }
    void persistWelcomeProxyChoice();
    motionRunIdRef.current += 1;
    const motionRunId = motionRunIdRef.current;
    setMotionPhase("proxy-confirm");
    window.setTimeout(() => {
      if (motionRunIdRef.current !== motionRunId) {
        return;
      }
      setMotionPhase("proxy-exit");
    }, 220);
    window.setTimeout(() => {
      if (motionRunIdRef.current !== motionRunId) {
        return;
      }
      setStep("dependencies");
      setMotionPhase("dependencies-enter");
    }, 780);
    window.setTimeout(() => {
      if (motionRunIdRef.current !== motionRunId) {
        return;
      }
      setMotionPhase("idle");
    }, 1140);
  };

  const handleDependencyBack = () => {
    if (dependencyNavigationLocked || dependenciesInstalling) {
      return;
    }
    installRunIdRef.current += 1;
    dependencyInstallStartedAtRef.current.clear();
    motionRunIdRef.current += 1;
    const motionRunId = motionRunIdRef.current;
    setMotionPhase("dependencies-back");
    window.setTimeout(() => {
      if (motionRunIdRef.current !== motionRunId) {
        return;
      }
      setDependencyState(createInitialDependencyState(installedDependencyNames));
      setStep("proxy");
      setMotionPhase("proxy-return");
    }, 420);
    window.setTimeout(() => {
      if (motionRunIdRef.current !== motionRunId) {
        return;
      }
      setMotionPhase("idle");
    }, 780);
  };

  const handleDependencyNext = () => {
    if (dependencyNavigationLocked || dependenciesInstalling) {
      return;
    }
    transitionDependenciesToReady();
  };

  const startDependencyInstall = () => {
    if (dependencyNavigationLocked || dependenciesInstalling) {
      return;
    }
    const pendingDependencies = CORE_DEPENDENCIES.filter(
      (name) =>
        !installedDependencyNames.has(name) &&
        dependencyState[name].stage !== "done",
    );
    if (pendingDependencies.length === 0) {
      handleDependencyNext();
      return;
    }

    installRunIdRef.current += 1;
    motionRunIdRef.current += 1;
    const startedAt = Date.now();
    for (const name of pendingDependencies) {
      dependencyInstallStartedAtRef.current.set(name, startedAt);
    }
    setStep("dependencies");
    setMotionPhase("installing");
    setDependencyState((current) => {
      const next = createInitialDependencyState(installedDependencyNames);
      for (const name of CORE_DEPENDENCIES) {
        if (current[name]?.stage === "done") {
          next[name] = { progress: 100, stage: "done" };
        }
      }
      for (const name of pendingDependencies) {
        next[name] = {
          progress: Math.max(current[name]?.progress ?? 0, 1),
          stage: "installing",
        };
      }
      return next;
    });

    void Promise.all(
      pendingDependencies.map(async (name) => {
        try {
          await installDependency.mutateAsync({ name });
        } catch {
          setDependencyState((current) => ({
            ...current,
            [name]: {
              progress: current[name]?.progress ?? 0,
              stage: "error",
            },
          }));
          setMotionPhase("idle");
        }
      }),
    );
  };

  const handleDependencyPrimary = () => {
    if (allDependenciesDone) {
      handleDependencyNext();
      return;
    }
    startDependencyInstall();
  };

  if (!props.open) {
    return null;
  }

  const languageOptions: Array<WelcomeDropdownOption<SupportedLanguage>> = [
    { value: "en", label: text.common.languages.en },
    { value: "zh-CN", label: text.common.languages.zhCN },
  ];
  const themeOptions: Array<WelcomeDropdownOption<XiaThemePackId>> =
    XIA_THEME_PACKS.map((pack) => ({
      label: text.themePacks[pack.id].label,
      swatch: pack.preview,
      value: pack.id,
    }));

  return (
    <section
      className="welcome-screen wails-drag"
      data-step={step}
      data-motion={motionPhase}
      aria-label={welcomeText.title}
    >
      <div className="welcome-scene" aria-hidden="true">
        <div className="welcome-moon">
          <Moon className="h-16 w-16" />
        </div>
        <div className="welcome-sun" />
        <div className="welcome-stars">
          {WELCOME_STARS.map((star, index) => (
            <span
              key={index}
              style={
                {
                  "--star-max-opacity": star.maxOpacity,
                  "--star-min-opacity": star.minOpacity,
                  animationDelay: star.delay,
                  animationDuration: star.duration,
                  height: star.size,
                  left: star.left,
                  top: star.top,
                  width: star.size,
                } as React.CSSProperties
              }
            />
          ))}
        </div>
        {WELCOME_SHOOTING_STARS.map((star, index) => (
          <div
            key={index}
            className="welcome-shooting-star"
            style={
              {
                "--shoot-travel-x": star.travelX,
                "--shoot-travel-y": star.travelY,
                animationDelay: star.delay,
                animationDuration: star.duration,
                left: star.left,
                top: star.top,
                width: star.width,
              } as React.CSSProperties
            }
          />
        ))}
        <div className="welcome-cloud welcome-cloud-a">
          <Cloud className="h-14 w-24" />
        </div>
        <div className="welcome-cloud welcome-cloud-b">
          <Cloud className="h-12 w-20" />
        </div>
        <div className="welcome-cloud welcome-cloud-c">
          <Cloud className="h-10 w-16" />
        </div>
        <div className="welcome-horizon-glow" />
        <div className="welcome-ocean">
          <div className="welcome-moon-reflection">
            <span />
            <span />
            <span />
            <span />
            <span />
            <span />
            <span />
          </div>
          <div className="welcome-star-reflections">
            <span />
            <span />
            <span />
            <span />
            <span />
          </div>
          <div className="welcome-depth-haze" />
          <div className="welcome-ripple-perspective">
            <span />
            <span />
            <span />
            <span />
            <span />
            <span />
            <span />
            <span />
            <span />
            <span />
          </div>
          <div className="welcome-water-texture welcome-water-texture-a" />
          <div className="welcome-water-texture welcome-water-texture-b" />
        </div>
        <div className="welcome-sand">
          <div className="welcome-shell welcome-shell-a" />
          <div className="welcome-shell welcome-shell-b" />
          <div className="welcome-shell welcome-shell-c" />
          <div className="welcome-sandcastle">
            <span className="welcome-sandcastle-tower welcome-sandcastle-tower-a" />
            <span className="welcome-sandcastle-tower welcome-sandcastle-tower-b" />
            <span className="welcome-sandcastle-keep" />
            <span className="welcome-sandcastle-gate" />
          </div>
          <div className="welcome-crab">
            <span className="welcome-crab-leg welcome-crab-leg-a" />
            <span className="welcome-crab-leg welcome-crab-leg-b" />
            <span className="welcome-crab-claw welcome-crab-claw-a" />
            <span className="welcome-crab-claw welcome-crab-claw-b" />
          </div>
          <div className="welcome-sand-marks">
            <span />
            <span />
            <span />
            <span />
            <span />
          </div>
        </div>
        <div className="welcome-surf">
          <div className="welcome-shore-wetness" />
          <div className="welcome-backwash" />
          <div className="welcome-swash welcome-swash-a" />
          <div className="welcome-swash welcome-swash-b" />
          <div className="welcome-foam-bits">
            <span />
            <span />
            <span />
            <span />
            <span />
            <span />
          </div>
          <div className="welcome-wave welcome-wave-a" />
          <div className="welcome-wave welcome-wave-b" />
          <div className="welcome-wave welcome-wave-c" />
        </div>
        <div className="welcome-chicken-track">
          <div className="welcome-chicken">
            <WelcomePixelChicken />
          </div>
        </div>
      </div>

      <div className="welcome-toolbar wails-no-drag">
        <WelcomeDropdown
          label={welcomeText.language}
          value={language}
          options={languageOptions}
          disabled={updateSettings.isPending}
          onChange={handleLanguageChange}
        />
        <WelcomeDropdown
          label={welcomeText.theme}
          value={themePackId}
          options={themeOptions}
          disabled={updateSettings.isPending}
          onChange={handleThemeChange}
        />
      </div>

      <div className="welcome-content wails-no-drag">
        <div className="welcome-step-panel" data-step={step}>
          {step === "proxy" ? (
            <div className="welcome-proxy-step" data-motion={motionPhase}>
              <div className="welcome-proxy-options">
                <button
                  type="button"
                  className={cn(
                    "welcome-choice",
                    proxyChoice === "none" && "is-selected",
                  )}
                  data-dimmed={
                    motionPhase === "proxy-exit" && proxyChoice !== "none"
                      ? "true"
                      : "false"
                  }
                  data-lifted={
                    motionPhase === "proxy-exit" && proxyChoice === "none"
                      ? "true"
                      : "false"
                  }
                  data-confirming={
                    motionPhase === "proxy-confirm" && proxyChoice === "none"
                      ? "true"
                      : "false"
                  }
                  disabled={proxyLocked}
                  onClick={() => setProxyChoice("none")}
                >
                  <span className="welcome-choice-icon">
                    <Waves className="h-6 w-6" />
                  </span>
                  <span>
                    <strong>{welcomeText.proxyNone}</strong>
                    <small>{welcomeText.noProxyDescription}</small>
                  </span>
                </button>
                <button
                  type="button"
                  className={cn(
                    "welcome-choice",
                    proxyChoice === "system" && "is-selected",
                  )}
                  data-dimmed={
                    motionPhase === "proxy-exit" && proxyChoice !== "system"
                      ? "true"
                      : "false"
                  }
                  data-lifted={
                    motionPhase === "proxy-exit" && proxyChoice === "system"
                      ? "true"
                      : "false"
                  }
                  data-confirming={
                    motionPhase === "proxy-confirm" && proxyChoice === "system"
                      ? "true"
                      : "false"
                  }
                  disabled={proxyLocked}
                  onClick={() => setProxyChoice("system")}
                >
                  <span className="welcome-choice-icon">
                    <Globe2 className="h-6 w-6" />
                  </span>
                  <span>
                    <strong>{welcomeText.proxySystem}</strong>
                    <small>{welcomeText.systemProxyDescription}</small>
                  </span>
                </button>
              </div>

              <div
                className="welcome-system-proxy"
                data-visible={proxyChoice === "system"}
              >
                <span>{welcomeText.systemProxyTitle}</span>
                <strong>{systemProxyDisplay}</strong>
              </div>

              <div className="welcome-actions">
                <button
                  type="button"
                  className="welcome-primary-action"
                  disabled={proxyLocked}
                  onClick={handleProxyNext}
                >
                  {text.actions.next}
                </button>
              </div>
            </div>
          ) : null}

          {step === "dependencies" ? (
            <div className="welcome-dependency-step" data-motion={motionPhase}>
              <div className="welcome-dependencies">
                {CORE_DEPENDENCIES.map((name) => {
                  const item = dependencyState[name];
                  const meta = DEPENDENCY_META[name];
                  const Icon = meta.Icon;
                  const update = dependencyUpdatesByName.get(name);
                  const latestVersion = dependencyLatestVersionLabel(
                    update?.latestVersion ||
                      update?.recommendedVersion ||
                      update?.upstreamVersion,
                    text.settings.checking,
                    text.dependencies.noRemoteVersionInfo,
                    dependencyUpdatesQuery.isLoading || dependencyUpdatesQuery.isFetching,
                  );
                  return (
                    <div
                      key={name}
                      className="welcome-dependency-chip"
                      data-stage={item.stage}
                      style={dependencyProgressStyle(item.progress)}
                    >
                      <span className="welcome-dependency-sparks" aria-hidden="true">
                        <span />
                        <span />
                        <span />
                      </span>
                      <div className="welcome-dependency-ring">
                        <Icon className="h-8 w-8" />
                      </div>
                      <strong>{meta.displayName}</strong>
                      <small>
                        {item.stage === "done"
                          ? text.dependencies.installed
                          : item.stage === "error"
                            ? text.dependencies.invalid
                            : item.stage === "installing"
                              ? `${welcomeText.installing} ${item.progress}%`
                              : latestVersion}
                      </small>
                    </div>
                  );
                })}
              </div>

              <div className="welcome-actions welcome-actions-split">
                <button
                  type="button"
                  className="welcome-secondary-action"
                  disabled={dependencyNavigationLocked || dependenciesInstalling}
                  onClick={handleDependencyBack}
                >
                  {text.actions.back}
                </button>
                <button
                  type="button"
                  className="welcome-primary-action"
                  onClick={handleDependencyPrimary}
                  disabled={dependenciesInstalling || dependencyNavigationLocked}
                >
                  {allDependenciesDone ? text.actions.next : welcomeText.installAll}
                </button>
              </div>
            </div>
          ) : null}

          {step === "ready" ? (
            <div className="welcome-ready-step">
              <div className="welcome-step-heading">
                <h1>{welcomeText.readyTitle}</h1>
                <p>{welcomeText.readyHint}</p>
              </div>
              <div className="welcome-actions">
                <button
                  type="button"
                  className="welcome-primary-action"
                  onClick={props.onComplete}
                >
                  {welcomeText.enter}
                </button>
              </div>
            </div>
          ) : null}
        </div>
      </div>

      <div className="welcome-brand wails-no-drag">
        <span className="welcome-brand-mark">
          <Egg className="h-4 w-4" />
        </span>
        <span>{text.appName}</span>
      </div>

      <button
        type="button"
        className="welcome-volume-button wails-no-drag"
        data-playing={bgm.playing ? "true" : "false"}
        onClick={bgm.toggle}
        aria-label={bgm.playing ? welcomeText.bgmOn : welcomeText.bgmOff}
        title={bgm.playing ? welcomeText.bgmOn : welcomeText.bgmOff}
      >
        {bgm.playing ? <Volume2 className="h-5 w-5" /> : <VolumeX className="h-5 w-5" />}
      </button>
    </section>
  );
}
