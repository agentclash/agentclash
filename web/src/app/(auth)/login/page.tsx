"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/lib/stores/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { v4 as uuidv4 } from "uuid";

const PRESETS = {
  dev: {
    userId: "11111111-1111-1111-1111-111111111111",
    email: "dev@agentclash.dev",
    displayName: "Dev User",
    workspaceMemberships: "22222222-2222-2222-2222-222222222222:admin",
  },
};

export default function LoginPage() {
  const router = useRouter();
  const { login } = useAuthStore();
  const [form, setForm] = useState({
    userId: "",
    email: "",
    displayName: "",
    workspaceMemberships: "",
  });
  const [error, setError] = useState("");

  function applyPreset() {
    setForm(PRESETS.dev);
  }

  function generateUUID() {
    setForm((f) => ({ ...f, userId: uuidv4() }));
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (!form.userId) {
      setError("User ID is required");
      return;
    }
    if (!form.workspaceMemberships) {
      setError("At least one workspace membership is required");
      return;
    }

    login(form);
    router.push("/");
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-10">
          <h1 className="font-[family-name:var(--font-display)] text-4xl mb-3">
            agent<span className="text-ds-accent">clash</span>
          </h1>
          <p className="text-sm text-text-2">
            Development authentication
          </p>
        </div>

        <div className="rounded-xl border border-border bg-card p-6">
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-sm font-medium text-text-1">Dev Auth</h2>
            <Button variant="outline" size="sm" onClick={applyPreset}>
              Use preset
            </Button>
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="userId" className="text-xs text-text-2">
                User ID (UUID)
              </Label>
              <div className="flex gap-2">
                <Input
                  id="userId"
                  value={form.userId}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, userId: e.target.value }))
                  }
                  placeholder="11111111-1111-1111-1111-111111111111"
                  className="font-[family-name:var(--font-mono)] text-xs"
                />
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={generateUUID}
                >
                  Gen
                </Button>
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="email" className="text-xs text-text-2">
                Email (optional)
              </Label>
              <Input
                id="email"
                type="email"
                value={form.email}
                onChange={(e) =>
                  setForm((f) => ({ ...f, email: e.target.value }))
                }
                placeholder="dev@agentclash.dev"
                className="text-xs"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="displayName" className="text-xs text-text-2">
                Display Name (optional)
              </Label>
              <Input
                id="displayName"
                value={form.displayName}
                onChange={(e) =>
                  setForm((f) => ({ ...f, displayName: e.target.value }))
                }
                placeholder="Dev User"
                className="text-xs"
              />
            </div>

            <div className="space-y-2">
              <Label
                htmlFor="workspaceMemberships"
                className="text-xs text-text-2"
              >
                Workspace Memberships
              </Label>
              <Input
                id="workspaceMemberships"
                value={form.workspaceMemberships}
                onChange={(e) =>
                  setForm((f) => ({
                    ...f,
                    workspaceMemberships: e.target.value,
                  }))
                }
                placeholder="workspace-uuid:role,workspace-uuid:role"
                className="font-[family-name:var(--font-mono)] text-xs"
              />
              <p className="text-[11px] text-text-3">
                Format: uuid:role — comma separated for multiple
              </p>
            </div>

            {error && (
              <p className="text-xs text-status-fail">{error}</p>
            )}

            <Button type="submit" className="w-full">
              Sign in
            </Button>
          </form>
        </div>

        <p className="text-center text-[11px] text-text-3 mt-6">
          This is a development authenticator. In production, WorkOS AuthKit handles
          authentication.
        </p>
      </div>
    </div>
  );
}
