"use client";
import { Button } from "@/components/ui/button";
import type { Requirement } from "@/lib/vibe";

export function Requirements({ requirements, busy, onRequirement }: {
  requirements: Requirement[];
  busy: boolean;
  onRequirement: (id: string, status: "accepted" | "rejected" | "superseded", statement?: string) => void;
}) {
  return (<>
        {requirements.filter(
          (r) => r.status !== "rejected" && r.status !== "superseded",
        ).length > 0 && (
          <div>
            <h3 className="mb-2 text-xs font-medium">Requirements</h3>
            {requirements
              .filter(
                (r) => r.status !== "rejected" && r.status !== "superseded",
              )
              .map((requirement) => (
                <div
                  key={requirement.id}
                  className="border-b border-builder-border py-3 text-xs leading-5"
                >
                  <p>{requirement.statement}</p>
                  <p className="mt-1 text-builder-fg-muted">
                    {requirement.status === "accepted"
                      ? "Confirmed by you"
                      : "Proposed · needs your confirmation"}
                  </p>
                  {requirement.status === "accepted" && (
                    <details className="mt-2">
                      <summary className="cursor-pointer text-builder-fg-muted">
                        Update requirement
                      </summary>
                      <form
                        className="mt-2 space-y-2"
                        onSubmit={(e) => {
                          e.preventDefault();
                          const data = new FormData(e.currentTarget);
                          onRequirement(
                            requirement.id,
                            "superseded",
                            String(data.get("replacement")),
                          );
                        }}
                      >
                        <textarea
                          name="replacement"
                          aria-label="Replacement requirement"
                          defaultValue={requirement.statement}
                          maxLength={4096}
                          required
                          className="w-full rounded border border-builder-border bg-builder-surface p-2"
                        />
                        <Button type="submit" size="sm" disabled={busy}>
                          Confirm replacement
                        </Button>
                      </form>
                    </details>
                  )}
                  {requirement.status === "proposed" && (
                    <div className="mt-2 flex gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={busy}
                        onClick={() =>
                          onRequirement(requirement.id, "accepted")
                        }
                      >
                        Confirm
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={busy}
                        onClick={() =>
                          onRequirement(requirement.id, "rejected")
                        }
                      >
                        Dismiss
                      </Button>
                    </div>
                  )}
                </div>
              ))}
          </div>
        )}
  </>);
}
