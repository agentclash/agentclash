import { Headphones } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import {
  humanVoiceMode,
  normalizedVoiceValue,
  VOICE_TRANSPORT_TEXT_SIM,
} from "@/lib/voice-evals";

interface VoiceModeBadgesProps {
  modality?: string;
  mode?: string;
  transport?: string;
  transports?: string[];
}

export function VoiceModeBadges({
  modality,
  mode,
  transport,
  transports,
}: VoiceModeBadgesProps) {
  const normalizedModality = normalizedVoiceValue(modality);
  const normalizedTransport = normalizedVoiceValue(transport);
  const normalizedTransports = (transports ?? []).map(normalizedVoiceValue);
  const hasTextSimTransport =
    normalizedTransport === VOICE_TRANSPORT_TEXT_SIM ||
    normalizedTransports.includes(VOICE_TRANSPORT_TEXT_SIM);

  if (
    normalizedModality !== "voice" &&
    !mode &&
    !normalizedTransport &&
    normalizedTransports.length === 0
  ) {
    return null;
  }

  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      {normalizedModality === "voice" && (
        <Badge variant="outline" className="gap-1">
          <Headphones className="size-3" />
          Voice
        </Badge>
      )}
      {mode && <Badge variant="secondary">{humanVoiceMode(mode)}</Badge>}
      {hasTextSimTransport && (
        <Badge variant="outline">text_sim</Badge>
      )}
    </span>
  );
}
