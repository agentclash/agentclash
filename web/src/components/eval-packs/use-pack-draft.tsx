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

interface State {
  composition: Composition;
  selection: BuilderSelection;
  saving: boolean;
  savedAt: number | null;
  compiling: boolean;
  compile: CompileDraftResponse | null;
  publishing: boolean;
}

type Action =
  | { type: "update"; updater: (composition: Composition) => Composition }
  | { type: "select"; selection: BuilderSelection }
  | { type: "savingStart" }
  | { type: "savingDone" }
  | { type: "compileStart" }
  | { type: "compileDone"; result: CompileDraftResponse }
  | { type: "compileFailed" }
  | { type: "publishingStart" }
  | { type: "publishingDone" };

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "update":
      return { ...state, composition: action.updater(state.composition) };
    case "select":
      return { ...state, selection: action.selection };
    case "savingStart":
      return { ...state, saving: true };
    case "savingDone":
      return { ...state, saving: false, savedAt: Date.now() };
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

  const [state, dispatch] = useReducer(reducer, {
    composition: initialComposition,
    selection: { section: "overview" },
    saving: false,
    savedAt: null,
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
        dispatch({ type: "savingDone" });
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
      toast.success("Eval pack published");
      // Re-enable the button before navigating so an intercepted navigation
      // (redirect, route guard) can't leave Publish permanently disabled.
      dispatch({ type: "publishingDone" });
      router.push(`/workspaces/${workspaceId}/eval-packs/${result.eval_pack_id}`);
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
