"use client";

import { useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

interface Field {
  key: string;
  label: string;
  placeholder?: string;
  required?: boolean;
  type?: "text" | "select" | "textarea" | "json";
  options?: { value: string; label: string }[];
}

interface CreateResourceDialogProps {
  title: string;
  description: string;
  endpoint: string;
  fields: Field[];
  buttonLabel?: string;
  children?: ReactNode;
}

export function CreateResourceDialog({
  title,
  description,
  endpoint,
  fields,
  buttonLabel = "Create",
}: CreateResourceDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  function setValue(key: string, val: string) {
    setValues((prev) => ({ ...prev, [key]: val }));
  }

  async function handleSubmit() {
    for (const f of fields) {
      if (f.required && !values[f.key]?.trim()) {
        toast.error(`${f.label} is required`);
        return;
      }
    }

    const body: Record<string, unknown> = {};
    for (const f of fields) {
      const val = values[f.key]?.trim();
      if (!val) continue;
      if (f.type === "json") {
        try {
          body[f.key] = JSON.parse(val);
        } catch {
          toast.error(`Invalid JSON in ${f.label}`);
          return;
        }
      } else {
        body[f.key] = val;
      }
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post(endpoint, body);
      toast.success("Created successfully");
      setOpen(false);
      setValues({});
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create");
    } finally {
      setSubmitting(false);
    }
  }

  const inputClass =
    "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        {buttonLabel}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          {fields.map((f) => (
            <div key={f.key}>
              <label className="mb-1.5 block text-sm font-medium">
                {f.label}
                {!f.required && (
                  <span className="text-muted-foreground font-normal"> (optional)</span>
                )}
              </label>
              {f.type === "select" && f.options ? (
                <select
                  value={values[f.key] || ""}
                  onChange={(e) => setValue(f.key, e.target.value)}
                  className={inputClass}
                >
                  <option value="">Select...</option>
                  {f.options.map((o) => (
                    <option key={o.value} value={o.value}>{o.label}</option>
                  ))}
                </select>
              ) : f.type === "json" || f.type === "textarea" ? (
                <textarea
                  value={values[f.key] || ""}
                  onChange={(e) => setValue(f.key, e.target.value)}
                  placeholder={f.placeholder}
                  rows={4}
                  spellCheck={false}
                  className={`${inputClass} font-[family-name:var(--font-mono)] text-xs resize-y`}
                />
              ) : (
                <input
                  type="text"
                  value={values[f.key] || ""}
                  onChange={(e) => setValue(f.key, e.target.value)}
                  placeholder={f.placeholder}
                  className={inputClass}
                />
              )}
            </div>
          ))}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)} disabled={submitting}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={submitting}>
            {submitting ? <Loader2 className="size-4 animate-spin" /> : buttonLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
