"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ToggleGroup } from "@/components/ui/toggle-group";
import { Loader2, Plus, Trash2, Rocket } from "lucide-react";
import type { ProviderAccount, ProviderConnectionModel } from "@/lib/api/types";

interface ModelEntry {
  id: string;
  providerAccountId: string;
  model: string;
  name: string;
}

let nextEntryId = 0;
function createEntry(
  providerAccounts: ProviderAccount[],
  models: ProviderConnectionModel[],
): ModelEntry {
  return {
    id: `entry-${nextEntryId++}`,
    providerAccountId: providerAccounts[0]?.id ?? "",
    model: models[0]?.id ?? "",
    name: "",
  };
}

interface ExperimentLauncherProps {
  providerAccounts: ProviderAccount[];
  models: ProviderConnectionModel[];
  onLaunchSingle: (data: {
    name: string;
    providerAccountId: string;
    model: string;
  }) => Promise<void>;
  onLaunchBatch: (data: {
    models: { providerAccountId: string; model: string; name: string }[];
  }) => Promise<void>;
}

export function ExperimentLauncher({
  providerAccounts,
  models,
  onLaunchSingle,
  onLaunchBatch,
}: ExperimentLauncherProps) {
  const [mode, setMode] = useState<"single" | "multi">("single");
  const [singleName, setSingleName] = useState("");
  const [singleProvider, setSingleProvider] = useState(
    providerAccounts[0]?.id ?? "",
  );
  const [singleModel, setSingleModel] = useState(models[0]?.id ?? "");
  const [entries, setEntries] = useState<ModelEntry[]>(() => [
    createEntry(providerAccounts, models),
    createEntry(providerAccounts, models),
  ]);
  const [launching, setLaunching] = useState(false);

  function addEntry() {
    setEntries((prev) => [
      ...prev,
      createEntry(providerAccounts, models),
    ]);
  }

  function removeEntry(id: string) {
    setEntries((prev) => prev.filter((e) => e.id !== id));
  }

  function updateEntry(id: string, field: keyof ModelEntry, value: string) {
    setEntries((prev) =>
      prev.map((e) => (e.id === id ? { ...e, [field]: value } : e)),
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLaunching(true);
    try {
      if (mode === "single") {
        await onLaunchSingle({
          name: singleName,
          providerAccountId: singleProvider,
          model: singleModel,
        });
        setSingleName("");
      } else {
        await onLaunchBatch({
          models: entries.map((entry) => ({
            providerAccountId: entry.providerAccountId,
            model: entry.model,
            name: entry.name,
          })),
        });
      }
    } finally {
      setLaunching(false);
    }
  }

  const hasEmptyModels =
    mode === "multi" &&
    entries.some((e) => !e.providerAccountId || !e.model);
  const canSubmit =
    !launching &&
    (mode === "single"
      ? !!singleProvider && !!singleModel
      : entries.length >= 2 && !hasEmptyModels);

  return (
    <form
      onSubmit={handleSubmit}
      className="rounded-lg border border-border p-4 space-y-4"
    >
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">Launch Experiment</h3>
        <ToggleGroup
          options={[
            { value: "single" as const, label: "Single Model" },
            { value: "multi" as const, label: "Multi-Model" },
          ]}
          value={mode}
          onChange={setMode}
        />
      </div>

      {mode === "single" ? (
        <div className="grid gap-4 md:grid-cols-3">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Name (optional)
            </label>
            <Input
              value={singleName}
              onChange={(e) => setSingleName(e.target.value)}
              placeholder="My experiment"
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Provider Account
            </label>
            <Select
              value={singleProvider}
              onValueChange={(v) => v && setSingleProvider(v)}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select provider" />
              </SelectTrigger>
              <SelectContent>
                {providerAccounts.map((a) => (
                  <SelectItem key={a.id} value={a.id}>
                    {a.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Model
            </label>
            <Select
              value={singleModel}
              onValueChange={(v) => v && setSingleModel(v)}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select model" />
              </SelectTrigger>
              <SelectContent>
                {models.map((m) => (
                  <SelectItem key={m.id} value={m.id}>
                    {m.display_name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {entries.map((entry, idx) => (
            <div
              key={entry.id}
              className="grid items-end gap-3 md:grid-cols-4"
            >
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">
                  Label
                </label>
                <Input
                  value={entry.name}
                  onChange={(e) =>
                    updateEntry(entry.id, "name", e.target.value)
                  }
                  placeholder={`Model ${idx + 1}`}
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">
                  Provider
                </label>
                <Select
                  value={entry.providerAccountId}
                  onValueChange={(v) =>
                    v && updateEntry(entry.id, "providerAccountId", v)
                  }
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {providerAccounts.map((a) => (
                      <SelectItem key={a.id} value={a.id}>
                        {a.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">
                  Model
                </label>
                <Select
                  value={entry.model}
                  onValueChange={(v) =>
                    v && updateEntry(entry.id, "model", v)
                  }
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {models.map((m) => (
                      <SelectItem key={m.id} value={m.id}>
                        {m.display_name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => removeEntry(entry.id)}
                disabled={entries.length <= 2}
                className="self-end"
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ))}
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={addEntry}
          >
            <Plus className="mr-1.5 size-3.5" />
            Add Model
          </Button>
        </div>
      )}

      <div className="flex justify-end">
        <Button type="submit" disabled={!canSubmit}>
          {launching ? (
            <Loader2 className="mr-2 size-4 animate-spin" />
          ) : (
            <Rocket className="mr-2 size-4" />
          )}
          {launching
            ? "Launching..."
            : mode === "single"
              ? "Launch Experiment"
              : `Launch ${entries.length} Experiments`}
        </Button>
      </div>
    </form>
  );
}
