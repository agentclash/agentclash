"use client";

// The pack builder's single source of truth: one cohesive draft document
// (a Composition) edited through a reducer, with debounced server-side autosave
// + compile. Cross-piece references (a scorecard dimension pointing at a
// validator/judge) need the whole draft in scope, so it lives in Context.

import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { useRouter } from "next/navigation";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type ReactNode,
} from "react";
import { toast } from "sonner";

import { ApiError } from "@/lib/api/errors";
import { compileDraft, patchDraft, publishDraft } from "./lib/api";
import type { CompileDraftResponse, Composition, PieceKind } from "./lib/types";

const AUTOSAVE_DELAY_MS = 700;

export type BuilderSelection =
  | { section: "overview" }
  | { section: "scorecard" }
  | { section: "piece"; kind: PieceKind; index: number };

export type SaveState =
  | { status: "idle" }
  | { status: "saving" }
  | { status: "saved"; at: number }
  | { status: "failed" };

export interface State {
  composition: Composition;
  selection: BuilderSelection;
  save: SaveState;
  compiling: boolean;
  compile: CompileDraftResponse | null;
  publishing: boolean;
}

export type Action =
  | { type: "update"; updater: (composition: Composition) => Composition }
  | { type: "select"; selection: BuilderSelection }
  | { type: "savingStart" }
  | { type: "savingDone" }
  | { type: "savingFailed" }
  | { type: "compileStart" }
  | { type: "compileDone"; result: CompileDraftResponse }
  | { type: "compileFailed" }
  | { type: "publishingStart" }
  | { type: "publishingDone" };

export function packDraftReducer(state: State, action: Action): State {
  switch (action.type) {
    case "update":
      return { ...state, composition: action.updater(state.composition) };
    case "select":
      return { ...state, selection: action.selection };
    case "savingStart":
      return { ...state, save: { status: "saving" } };
    case "savingDone":
      return { ...state, save: { status: "saved", at: Date.now() } };
    case "savingFailed":
      return { ...state, save: { status: "failed" } };
    case "compileStart":
      return { ...state, compiling: true };
    case "compileDone":
      return { ...state, compiling: false, compile: action.result };
    case "compileFailed":
      return { ...state, compiling: false };
    case "publishingStart":
      return { ...state, publishing: true };
    case "publishingDone":
      return { ...state, publishing: false };
    default:
      return state;
  }
}

/** Header status line for the builder. */
export function saveStatusLabel(state: State): string {
  if (state.save.status === "saving") return "Saving…";
  if (state.compiling) return "Updating preview…";
  if (state.save.status === "failed") return "Not saved";
  if (state.save.status === "saved") return "Saved";
  return "";
}

/**
 * Publish waits for in-flight saves and compiles: a save landing between the
 * publish patch and the publish call would publish an older composition, and
 * a running compile means the validity shown is stale.
 */
export function publishDisabled(state: State): boolean {
  const invalid = state.compile != null && !state.compile.valid;
  return (
    state.publishing ||
    state.save.status === "saving" ||
    state.compiling ||
    invalid
  );
}

interface PackDraftContextValue {
  state: State;
  workspaceId: string;
  select: (selection: BuilderSelection) => void;
  update: (updater: (composition: Composition) => Composition) => void;
  publish: () => Promise<void>;
}

const PackDraftContext = createContext<PackDraftContextValue | null>(null);

export function usePackDraft(): PackDraftContextValue {
  const ctx = useContext(PackDraftContext);
  if (!ctx) {
    throw new Error("usePackDraft must be used within a PackDraftProvider");
  }
  return ctx;
}

export function PackDraftProvider({
  workspaceId,
  draftId,
  initialComposition,
  children,
}: {
  workspaceId: string;
  draftId: string;
  initialComposition: Composition;
  children: ReactNode;
}) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const [state, dispatch] = useReducer(packDraftReducer, {
    composition: initialComposition,
    selection: { section: "overview" },
    save: { status: "idle" },
    compiling: false,
    compile: null,
    publishing: false,
  });

  // Latest composition for the user-triggered publish callback. Synced via an
  // effect — never written during render.
  const compositionRef = useRef(state.composition);
  const dirtyRef = useRef(false);

  useEffect(() => {
    compositionRef.current = state.composition;
  }, [state.composition]);

  const runCompile = useCallback(async () => {
    dispatch({ type: "compileStart" });
    try {
      const token = await getAccessToken();
      const result = await compileDraft(token, workspaceId, draftId);
      dispatch({ type: "compileDone", result });
    } catch {
      dispatch({ type: "compileFailed" });
    }
  }, [getAccessToken, workspaceId, draftId]);

  // Compile once on mount so the spec card reflects the loaded draft.
  useEffect(() => {
    void runCompile();
  }, [runCompile]);

  // Debounced save + recompile whenever the composition changes (after mount).
  useEffect(() => {
    if (!dirtyRef.current) {
      dirtyRef.current = true;
      return;
    }
    const handle = setTimeout(async () => {
      dispatch({ type: "savingStart" });
      try {
        const token = await getAccessToken();
        await patchDraft(token, workspaceId, draftId, { composition: state.composition });
        dispatch({ type: "savingDone" });
        await runCompile();
      } catch (err) {
        dispatch({ type: "savingFailed" });
        toast.error(err instanceof ApiError ? err.message : "Failed to save draft");
      }
    }, AUTOSAVE_DELAY_MS);
    return () => clearTimeout(handle);
  }, [state.composition, getAccessToken, workspaceId, draftId, runCompile]);

  const select = useCallback((selection: BuilderSelection) => {
    dispatch({ type: "select", selection });
  }, []);

  const update = useCallback((updater: (composition: Composition) => Composition) => {
    dispatch({ type: "update", updater });
  }, []);

  const publish = useCallback(async () => {
    dispatch({ type: "publishingStart" });
    try {
      const token = await getAccessToken();
      // Persist the latest edits before publishing so the snapshot is current.
      await patchDraft(token, workspaceId, draftId, { composition: compositionRef.current });
      const result = await publishDraft(token, workspaceId, draftId);
      toast.success("Challenge pack published");
      // Re-enable the button before navigating so an intercepted navigation
      // (redirect, route guard) can't leave Publish permanently disabled.
      dispatch({ type: "publishingDone" });
      router.push(`/workspaces/${workspaceId}/challenge-packs/${result.challenge_pack_id}`);
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message || "Publish failed — resolve the validation issues first");
      } else {
        toast.error("Publish failed");
      }
      dispatch({ type: "publishingDone" });
    }
  }, [getAccessToken, workspaceId, draftId, router]);

  const value = useMemo<PackDraftContextValue>(
    () => ({ state, workspaceId, select, update, publish }),
    [state, workspaceId, select, update, publish],
  );

  return <PackDraftContext.Provider value={value}>{children}</PackDraftContext.Provider>;
}
