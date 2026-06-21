"use client";

import Link from "next/link";
import {
  Boxes,
  ChevronLeft,
  HelpCircle,
  Loader2,
  Maximize2,
  MessageSquareDashed,
  Minimize2,
  PencilRuler,
  Wrench,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { CanvasNodeKind } from "../lib/graph";

type AddableKind = Exclude<CanvasNodeKind, "inputs">;

export interface EditorToolbarProps {
  mode: "create" | "edit";
  name: string;
  onNameChange: (value: string) => void;
  onAdd: (kind: AddableKind) => void;
  onStartTour: () => void;
  immersive: boolean;
  onToggleImmersive: () => void;
  formEditorHref: string;
  onExit: () => void;
  onSave: () => void;
  canSave: boolean;
  saving: boolean;
}

const palette: { kind: AddableKind; label: string; icon: typeof Boxes }[] = [
  { kind: "operation", label: "Operation", icon: Boxes },
  { kind: "tool", label: "Tool", icon: Wrench },
  { kind: "canned", label: "Canned response", icon: MessageSquareDashed },
];

/**
 * The editor's top bar: an inline tool-name field on the left, the node palette
 * in the middle ("options to pick"), and actions on the right. Stateless — the
 * canvas owns all state and passes handlers down.
 */
export function EditorToolbar({
  mode,
  name,
  onNameChange,
  onAdd,
  onStartTour,
  immersive,
  onToggleImmersive,
  formEditorHref,
  onExit,
  onSave,
  canSave,
  saving,
}: EditorToolbarProps) {
  return (
    <TooltipProvider delay={200}>
      <div className="flex flex-shrink-0 items-center gap-2 border-b border-border bg-background px-3 py-2">
        <Tooltip>
          <TooltipTrigger
            render={
              <Button variant="ghost" size="icon-sm" onClick={onExit} aria-label="Back to tools" />
            }
          >
            <ChevronLeft className="size-4" />
          </TooltipTrigger>
          <TooltipContent side="bottom">Back to tools</TooltipContent>
        </Tooltip>

        <input
          value={name}
          onChange={(e) => onNameChange(e.target.value)}
          placeholder="Untitled tool"
          aria-label="Tool name"
          className="w-40 rounded-md bg-transparent px-2 py-1 text-sm font-medium outline-none placeholder:text-muted-foreground/60 focus:bg-muted/50 sm:w-48"
        />

        <Separator orientation="vertical" className="mx-1 h-6" />

        <div data-tour="palette" className="flex items-center gap-1.5">
          {palette.map(({ kind, label, icon: Icon }) => (
            <Button key={kind} type="button" variant="outline" size="sm" onClick={() => onAdd(kind)}>
              <Icon data-icon="inline-start" className="size-3.5" />
              <span className="hidden sm:inline">{label}</span>
            </Button>
          ))}
        </div>

        <div className="flex-1" />

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={onStartTour}
                aria-label="Show guided tour"
              />
            }
          >
            <HelpCircle className="size-4" />
          </TooltipTrigger>
          <TooltipContent side="bottom">Guided tour</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={<Button variant="ghost" size="icon-sm" render={<Link href={formEditorHref} />} />}
          >
            <PencilRuler className="size-4" />
          </TooltipTrigger>
          <TooltipContent side="bottom">Use the form editor</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={onToggleImmersive}
                aria-label={immersive ? "Exit fullscreen" : "Enter fullscreen"}
              />
            }
          >
            {immersive ? <Minimize2 className="size-4" /> : <Maximize2 className="size-4" />}
          </TooltipTrigger>
          <TooltipContent side="bottom">{immersive ? "Exit fullscreen" : "Fullscreen"}</TooltipContent>
        </Tooltip>

        <Separator orientation="vertical" className="mx-1 h-6" />

        <Button data-tour="save" onClick={onSave} disabled={!canSave}>
          {saving ? <Loader2 className="size-4 animate-spin" /> : mode === "create" ? "Save tool" : "Save"}
        </Button>
      </div>
    </TooltipProvider>
  );
}
