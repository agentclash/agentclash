"use client";
import { CollectionNotice } from "./collection-notice";
import type { CollectionStatus } from "@/lib/use-vibe-collection";
import { ArrowRight } from "lucide-react";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { savedWorkURL } from "@/lib/vibe-keep";
import type { SavedCheck } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

export function SavedWorkDialog({open, onOpenChange, testJourney, savedChecks, hasResults, onResults, status, onRetry}: {
  status?: CollectionStatus; onRetry?: () => void;
  open: boolean; onOpenChange: (open: boolean) => void; testJourney?: boolean;
  savedChecks: SavedCheck[]; hasResults: boolean; onResults: () => void;
}) {
  return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="vibe-workspace vibe-dialog">
          <DialogTitle>
            Your saved work
          </DialogTitle>
          <DialogDescription>
            {testJourney
              ? "Tests you can return to after your next change."
              : "Private checks you can return to and repeat."}
          </DialogDescription>
          <div className="vibe-dialog-body space-y-4">
          <CollectionNotice hasRows={savedChecks.length > 0} status={status} retry={onRetry} />
          {savedChecks.length ? (
            savedChecks.map((c) => (
              <a
                key={c.id}
                className="rounded-lg border p-3 text-sm"
                href={savedWorkURL(c)}
              >
                {c.title}
                <span className="mt-1 block text-xs text-muted-foreground">
                  {c.kind === "brief" ? "Preparation brief · not run" : c.draft_id
                    ? "Saved tests"
                    : c.source?.kind === "provided_conversations"
                      ? "Provided chats"
                      : "Text test"}
                </span>
              </a>
            ))
          ) : (!status || status === "ready") && (
            <p className="text-sm text-muted-foreground">
              {testJourney
                ? "Choose “Keep these tests” to find them here next time."
                : "After your first result, choose “Save this check” to keep it here."}
            </p>
          )}

          {hasResults && (
            <VibeButton
              onClick={() => {
                onOpenChange(false);
                onResults();
              }}
            >
              Results in this conversation
              <ArrowRight />
            </VibeButton>
          )}
          </div>
        </DialogContent>
      </Dialog>
  );
}
