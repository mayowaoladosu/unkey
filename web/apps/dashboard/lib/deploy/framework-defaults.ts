import type { FrameworkDetectionDefaults } from "@unkey/db/src/schema";
import type { FrameworkDetection } from "./framework-detection";

export type FrameworkDefaults = FrameworkDetectionDefaults;

/**
 * Converts advisory detection output into settings that can be reviewed and
 * explicitly applied. Ambiguous choices stay null instead of being guessed.
 */
export function resolveFrameworkDefaults(detection: FrameworkDetection): FrameworkDefaults {
  const rootDirectory = detection.rootCandidates.length === 1 ? detection.rootCandidates[0] : null;
  const recommendedDockerfile = detection.recommendedDockerfile;
  const unresolvedCodes = new Set(detection.unresolvedDecisions.map((decision) => decision.code));
  const dockerfile =
    recommendedDockerfile && rootDirectory && rootDirectory !== "."
      ? recommendedDockerfile.replace(new RegExp(`^${escapeRegex(rootDirectory)}/`), "")
      : recommendedDockerfile;
  const buildCommandIsUnambiguous =
    detection.buildStrategy === "zero-config" &&
    !unresolvedCodes.has("select-package-manager") &&
    !unresolvedCodes.has("select-root-directory") &&
    !unresolvedCodes.has("select-framework");

  return {
    rootDirectory,
    dockerfile,
    buildCommand: buildCommandIsUnambiguous ? detection.commandDefaults.buildCommand : null,
  };
}

export function hasFrameworkDefaults(defaults: FrameworkDefaults): boolean {
  return (
    (defaults.rootDirectory !== null && defaults.rootDirectory !== ".") ||
    defaults.dockerfile !== null ||
    defaults.buildCommand !== null
  );
}

/**
 * Returns true only for a detected static output that can be accepted without
 * changing build settings. Unknown or unresolved detections remain advisory.
 */
export function canAcceptDetectedOutput(detection: unknown, defaults: FrameworkDefaults): boolean {
  if (hasFrameworkDefaults(defaults) || !isRecord(detection)) {
    return false;
  }

  const preset = detection.preset;
  const output = detection.output;
  const unresolvedDecisions = detection.unresolvedDecisions;
  return (
    isRecord(preset) &&
    typeof preset.id === "string" &&
    preset.id.length > 0 &&
    isRecord(output) &&
    output.mode === "static" &&
    typeof output.directory === "string" &&
    output.directory.length > 0 &&
    Array.isArray(unresolvedDecisions) &&
    unresolvedDecisions.length === 0
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function escapeRegex(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
