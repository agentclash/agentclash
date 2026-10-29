"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import {advancedURL} from "@/lib/vibe-keep";
import {projectTitle} from "@/lib/vibe-project";
import { Paperclip } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { type Artifact, type Models, type Session, type VibeConfig } from "@/lib/vibe";
import { CreditsDialog } from "./credits-dialog";
import { ModelSelect } from "./model-select";
import { VibeButton } from "./vibe-button";

type Props = {
  reimport?: ReactNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  config: VibeConfig | null;
  session: Session | null;
  artifact?: Artifact;
  artifacts: Artifact[];
  models: Models;
  busy: boolean;
  workspace: string;
  testJourney: boolean;
  projectReturnPath?:string;
  savedDraft?: { workspace_id: string; draft_id: string } | null;
  onModels: (models: Models) => void;
  onVersion: (id: string) => void;
  onSave: () => void;
  onImport: () => void;
  onExportConversation: () => void;
};

export function AgentSettings({reimport,open, onOpenChange, config, session, artifact, artifacts, models, busy, workspace, testJourney, projectReturnPath,savedDraft, onModels, onVersion, onSave, onImport, onExportConversation}: Props) {
  const savable = !!artifact && artifact.kind !== "test_plan" && artifact.kind !== "conversation_evaluation";
  return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="vibe-workspace vibe-dialog">
          <DialogTitle>Settings and details</DialogTitle>
          <DialogDescription>
            Choose models, review versions, or export your work.
          </DialogDescription>
          <div className="vibe-dialog-body space-y-4">
          {(workspace || savable) && <div className="flex flex-wrap gap-2">
          {workspace && <CreditsDialog workspace={workspace} />}
          {savable && (
              <VibeButton
                disabled={busy}
                onClick={() => {
                  onOpenChange(false);
                  onSave();
                }}
              >
                {testJourney ? "Keep these tests" : "Save agent"}
              </VibeButton>
            )}
          </div>}
          {savedDraft && (
            <Link
              className="text-sm underline"
              href={advancedURL(savedDraft.workspace_id,savedDraft.draft_id,projectReturnPath,session?projectTitle(session):undefined)}
            >
              Open saved evaluation
            </Link>
          )}
          {(
            [
              ["Assistant", "assistant"],
              ["Agent", "target"],
              ["Evaluator", "evaluator"],
            ] as const
          )
            .filter(
              ([, role]) =>
                role !== "target" ||
                artifact?.kind !== "conversation_evaluation",
            )
            .map(([label, role]) => (
              <div key={role}>
                <ModelSelect
                  label={label}
                  value={models[role]}
                  models={config?.models || []}
                  disabled={
                    busy ||
                    (role === "evaluator" && session?.anonymous !== false &&
                      !(config?.local_testing && !config?.free_only))
                  }
                  onChange={(value) =>
                    onModels({ ...models, [role]: value })
                  }
                />
                <p className="mt-1 text-sm text-builder-fg-muted">
                  {role === "assistant"
                    ? "Plans the check and helps explain results."
                    : role === "target"
                      ? "Generates replies when testing instructions here. Changing it starts a fresh preview conversation."
                      : config?.local_testing && !config?.free_only
                        ? "Grades replies against your expectations. Changing it requires a new baseline or rechecking saved grades."
                        : "Grades replies against your expectations. The free trial keeps this fixed."}
                </p>
              </div>
            ))}
          {config?.local_testing && !config?.free_only && (
            <Button variant="outline" disabled={busy} onClick={() =>
              onModels({ assistant: models.assistant, target: models.assistant, evaluator: models.assistant })
            }>
              Use assistant model for all roles
            </Button>
          )}
          <p className="text-sm text-builder-fg-muted">
            Model changes apply to new messages and runs. Use “Retry with another model” to switch a failed request.
          </p>
          {artifacts.length > 1 && (
            <label className="text-sm">
              Agent version
              <select
                aria-label="Agent version"
                value={artifact?.id}
                disabled={busy}
                className="mt-2 w-full rounded-[var(--vibe-radius-sm)] border bg-background p-2"
                onChange={(e) => {
                  onVersion(e.target.value);
                }}
              >
                {artifacts.map((a, i) => (
                  <option key={a.id} value={a.id}>
                    Version {i + 1}: {a.title}
                  </option>
                ))}
              </select>
            </label>
          )}
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => onImport()}
          >
            <Paperclip size={16} /> Import a test pack
          </Button>
          {reimport}
          {(savable || session) && <div className="flex flex-wrap gap-2">
          {savable && (
              <Button
                variant="outline"
                onClick={onExportConversation}
              >
                {testJourney
                  ? "Export tests and agent instructions"
                  : "Export agent and checks"}
              </Button>
            )}
          {session && (
            <Button variant="outline" onClick={onExportConversation}>
              Export selected version / result
            </Button>
          )}
          </div>}
          <details className="text-sm">
            <summary className="cursor-pointer">
              Available capabilities and documentation
            </summary>
            {config?.capabilities?.map((c) => (
              <p key={c.id} className="mt-3">
                <strong>{c.label}{c.available === false ? " (unavailable)" : ""}:</strong> {c.description}{" "}
                {c.url && (
                  <a
                    href={c.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline"
                  >
                    {c.label} documentation ↗
                  </a>
                )}
              </p>
            ))}
          </details>
          </div>
        </DialogContent>
      </Dialog>
  );
}
