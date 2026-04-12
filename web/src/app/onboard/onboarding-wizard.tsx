"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { OnboardResult } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Loader2, ArrowRight, Sparkles } from "lucide-react";

type Step = "org" | "workspace";

export function OnboardingWizard() {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const [step, setStep] = useState<Step>("org");
  const [orgName, setOrgName] = useState("");
  const [workspaceName, setWorkspaceName] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit() {
    if (!workspaceName.trim()) return;

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await api.post<OnboardResult>("/v1/onboarding", {
        organization_name: orgName.trim(),
        workspace_name: workspaceName.trim(),
      });

      toast.success("You're all set!");
      router.push(`/workspaces/${result.workspace.id}`);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === "already_onboarded") {
          toast.error("You're already onboarded — redirecting...");
          router.push("/dashboard");
          return;
        }
        toast.error(err.message);
      } else {
        toast.error("Something went wrong. Please try again.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="w-full max-w-md px-6">
        {/* Progress indicator */}
        <div className="mb-8 flex items-center gap-2">
          <div
            className={`h-1 flex-1 rounded-full transition-colors ${
              step === "org" ? "bg-foreground" : "bg-foreground"
            }`}
          />
          <div
            className={`h-1 flex-1 rounded-full transition-colors ${
              step === "workspace" ? "bg-foreground" : "bg-muted"
            }`}
          />
        </div>

        {step === "org" && (
          <div>
            <div className="mb-1 flex items-center gap-2 text-muted-foreground">
              <Sparkles className="size-4" />
              <span className="text-xs font-medium uppercase tracking-wider">
                Step 1 of 2
              </span>
            </div>
            <h1 className="mb-2 text-2xl font-semibold tracking-tight">
              Name your organization
            </h1>
            <p className="mb-8 text-sm text-muted-foreground">
              This is your team or company. You can always change it later.
            </p>

            <label className="mb-2 block text-sm font-medium">
              Organization name
            </label>
            <input
              type="text"
              value={orgName}
              onChange={(e) => setOrgName(e.target.value)}
              placeholder="e.g. Acme Labs"
              autoFocus
              className="mb-6 block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
              onKeyDown={(e) => {
                if (e.key === "Enter" && orgName.trim()) setStep("workspace");
              }}
            />

            <Button
              className="w-full"
              disabled={!orgName.trim()}
              onClick={() => setStep("workspace")}
            >
              Continue
              <ArrowRight data-icon="inline-end" className="size-4" />
            </Button>
          </div>
        )}

        {step === "workspace" && (
          <div>
            <div className="mb-1 flex items-center gap-2 text-muted-foreground">
              <Sparkles className="size-4" />
              <span className="text-xs font-medium uppercase tracking-wider">
                Step 2 of 2
              </span>
            </div>
            <h1 className="mb-2 text-2xl font-semibold tracking-tight">
              Create your first workspace
            </h1>
            <p className="mb-8 text-sm text-muted-foreground">
              Workspaces hold your runs, builds, and challenge packs.
            </p>

            <label className="mb-2 block text-sm font-medium">
              Workspace name
            </label>
            <input
              type="text"
              value={workspaceName}
              onChange={(e) => setWorkspaceName(e.target.value)}
              placeholder="e.g. Production"
              autoFocus
              className="mb-6 block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
              onKeyDown={(e) => {
                if (e.key === "Enter" && workspaceName.trim()) handleSubmit();
              }}
            />

            <div className="flex gap-3">
              <Button
                variant="outline"
                className="flex-1"
                onClick={() => setStep("org")}
                disabled={submitting}
              >
                Back
              </Button>
              <Button
                className="flex-1"
                disabled={!workspaceName.trim() || submitting}
                onClick={handleSubmit}
              >
                {submitting ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  "Create workspace"
                )}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
