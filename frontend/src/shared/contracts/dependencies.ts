export type DependencyStatus = "missing" | "installed" | "invalid";
export type DependencyKind = "bin" | "runtime";

export interface Dependency {
  name: string;
  kind?: DependencyKind | string;
  execPath?: string;
  version?: string;
  status?: DependencyStatus | string;
  sourceKind?: string;
  sourceRef?: string;
  manager?: string;
  installedAt?: string;
  updatedAt?: string;
}

export interface DependencyUpdateInfo {
  name: string;
  latestVersion?: string;
  recommendedVersion?: string;
  upstreamVersion?: string;
  releaseNotes?: string;
  releaseNotesUrl?: string;
  autoUpdate?: boolean;
  required?: boolean;
}

export type DependencyInstallStage = "idle" | "downloading" | "extracting" | "verifying" | "done" | "error";

export interface DependencyInstallState {
  name: string;
  stage: DependencyInstallStage | string;
  progress: number;
  message?: string;
  updatedAt?: string;
}

export interface InstallDependencyRequest {
  name: string;
  version?: string;
  manager?: string;
}

export interface SetDependencyPathRequest {
  name: string;
  execPath: string;
}

export interface VerifyDependencyRequest {
  name: string;
}

export interface RemoveDependencyRequest {
  name: string;
}

export interface OpenDependencyDirectoryRequest {
  name: string;
}

export interface GetDependencyInstallStateRequest {
  name: string;
}
