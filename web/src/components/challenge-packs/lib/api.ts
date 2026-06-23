// Thin typed wrappers over the challenge-pack builder endpoints. Mutations go
// through createApiClient (with a fresh access token); reads use the SWR hooks
// in @/lib/api/swr against the same paths.

import { createApiClient } from "@/lib/api/client";
import type {
  ChallengePackDraft,
  ChallengePiece,
  CompileDraftResponse,
  Composition,
  ExecutionMode,
  InstantiateCatalogPackResponse,
  PieceKind,
} from "./types";

type Token = string | null | undefined;

export interface PublishDraftResponse {
  challenge_pack_id: string;
  challenge_pack_version_id: string;
  evaluation_spec_id: string;
  input_set_ids: string[];
  bundle_artifact_id?: string;
}

export function draftCollectionPath(workspaceId: string): string {
  return `/v1/workspaces/${workspaceId}/challenge-pack-drafts`;
}

export function draftPath(workspaceId: string, draftId: string): string {
  return `${draftCollectionPath(workspaceId)}/${draftId}`;
}

export function piecesPath(workspaceId: string): string {
  return `/v1/workspaces/${workspaceId}/challenge-pieces`;
}

export function piecePath(workspaceId: string, pieceId: string): string {
  return `${piecesPath(workspaceId)}/${pieceId}`;
}

export function pieceLibraryPath(): string {
  return "/v1/challenge-piece-library";
}

export async function createDraft(
  token: Token,
  workspaceId: string,
  body: { name: string; execution_mode?: ExecutionMode; composition?: Composition },
): Promise<ChallengePackDraft> {
  return createApiClient(token ?? undefined).post<ChallengePackDraft>(
    draftCollectionPath(workspaceId),
    body,
  );
}

export async function patchDraft(
  token: Token,
  workspaceId: string,
  draftId: string,
  body: { name?: string; execution_mode?: ExecutionMode; composition?: Composition },
): Promise<ChallengePackDraft> {
  return createApiClient(token ?? undefined).patch<ChallengePackDraft>(
    draftPath(workspaceId, draftId),
    body,
  );
}

export async function compileDraft(
  token: Token,
  workspaceId: string,
  draftId: string,
): Promise<CompileDraftResponse> {
  return createApiClient(token ?? undefined).post<CompileDraftResponse>(
    `${draftPath(workspaceId, draftId)}/compile`,
  );
}

export async function publishDraft(
  token: Token,
  workspaceId: string,
  draftId: string,
): Promise<PublishDraftResponse> {
  return createApiClient(token ?? undefined).post<PublishDraftResponse>(
    `${draftPath(workspaceId, draftId)}/publish`,
  );
}

export async function createPiece(
  token: Token,
  workspaceId: string,
  body: { kind: PieceKind; slug: string; name: string; description?: string; definition: unknown },
): Promise<ChallengePiece> {
  return createApiClient(token ?? undefined).post<ChallengePiece>(piecesPath(workspaceId), body);
}

// --- catalog (the curated library) ---
// Reads use the SWR hooks against these paths; the instantiate mutation goes
// through a fresh access token like the other authoring mutations above.

export function catalogPath(): string {
  return "/v1/challenge-pack-catalog";
}

export function catalogPackPath(slug: string): string {
  return `${catalogPath()}/${slug}`;
}

export function catalogInstantiatePath(workspaceId: string, slug: string): string {
  return `/v1/workspaces/${workspaceId}/challenge-pack-catalog/${slug}/instantiate`;
}

export async function instantiateCatalogPack(
  token: Token,
  workspaceId: string,
  slug: string,
): Promise<InstantiateCatalogPackResponse> {
  return createApiClient(token ?? undefined).post<InstantiateCatalogPackResponse>(
    catalogInstantiatePath(workspaceId, slug),
  );
}
