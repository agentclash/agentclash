import type { ChallengePack, ChallengePackVersion, Run } from "@/lib/api/types";

export const VOICE_MODALITY = "voice";
export const VOICE_MODE_TEXT_SIM = "text-sim";
export const VOICE_TRANSPORT_TEXT_SIM = "text_sim";

export function normalizedVoiceValue(value?: string): string {
  return value?.trim().toLowerCase() ?? "";
}

export function isVoiceVersion(version?: ChallengePackVersion | null): boolean {
  return normalizedVoiceValue(version?.modality) === VOICE_MODALITY;
}

export function versionSupportsTextSim(
  version?: ChallengePackVersion | null,
): boolean {
  if (!isVoiceVersion(version)) return false;
  return (version?.interface_transports ?? []).some(
    (transport) =>
      normalizedVoiceValue(transport) === VOICE_TRANSPORT_TEXT_SIM,
  );
}

export function latestChallengePackVersion(
  pack: ChallengePack,
): ChallengePackVersion | null {
  if (pack.versions.length === 0) return null;
  return pack.versions.reduce((left, right) =>
    left.version_number > right.version_number ? left : right,
  );
}

export function hasVoiceVersion(pack: ChallengePack): boolean {
  return pack.versions.some(isVoiceVersion);
}

export function isVoiceRun(run: Pick<Run, "modality" | "voice">): boolean {
  return (
    normalizedVoiceValue(run.modality) === VOICE_MODALITY ||
    normalizedVoiceValue(run.voice?.modality) === VOICE_MODALITY
  );
}

export function voiceRunMode(run: Pick<Run, "mode" | "voice">): string {
  return run.voice?.mode || run.mode || "";
}

export function voiceRunTransport(run: Pick<Run, "voice">): string {
  return run.voice?.transport || "";
}

export function humanVoiceMode(value?: string): string {
  switch (normalizedVoiceValue(value)) {
    case VOICE_MODE_TEXT_SIM:
      return "Text simulation";
    case "audio-sim":
      return "Audio simulation";
    case "live-call":
      return "Live call";
    case "replay-import":
      return "Replay import";
    default:
      return value || "Voice";
  }
}
