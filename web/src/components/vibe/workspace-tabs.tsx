"use client";

import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";
import { useId } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

export function WorkspaceTabs({
  testPlan,
  ready,
  scenarioCount,
  activeView,
}: {
  testPlan: boolean;
  ready: boolean;
  scenarioCount?: number;
  activeView: string;
}) {
  const indicatorID = useId();
  const reducedMotion = useReducedMotion();
  const transition = reducedMotion
    ? { duration: 0 }
    : { type: "spring" as const, stiffness: 420, damping: 36, mass: 0.8 };
  const tabs = [
    {
      value: "build",
      label: "Describe",
      help: "Tell the setup assistant what your agent should do, or ask it to change the instructions.",
    },
    ...(ready
      ? [
          {
            value: "try",
            label: testPlan ? "Text preview" : "Chat with agent",
            help: testPlan
              ? "Prepare a text simulation. Your existing agent is not connected."
              : "Send a message to the agent you created and continue the conversation.",
          },
          {
            value: "checks",
            label: testPlan ? "Test plan" : "Test replies",
            help: testPlan
              ? "Review and export situations to test in your own environment."
              : `Run ${scenarioCount || "the"} example situations and see whether each reply matches what you expected.`,
          },
        ]
      : []),
  ];

  return (
    <TooltipProvider delay={350}>
      <TabsPrimitive.List
        aria-label="Workspace views"
        className="relative inline-flex h-9 items-center gap-1"
        render={<motion.div layout={!reducedMotion} transition={transition} />}
      >
        <AnimatePresence initial={false}>
          {tabs.map((tab) => (
            <Tooltip key={tab.value}>
              <span id={`${indicatorID}-${tab.value}-help`} hidden>
                {tab.help}
              </span>
              <TabsPrimitive.Tab
                value={tab.value}
                aria-describedby={`${indicatorID}-${tab.value}-help`}
                className="relative isolate inline-flex h-8 flex-none items-center justify-center rounded-lg px-3 text-sm font-medium whitespace-nowrap text-builder-fg-muted transition-colors hover:text-builder-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-builder-border-strong data-active:text-builder-fg motion-reduce:transition-none"
                render={
                  <TooltipTrigger
                    render={
                      <motion.button
                        layout={reducedMotion ? false : "position"}
                        initial={reducedMotion ? false : { opacity: 0, x: -8 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{
                          ...transition,
                          opacity: { duration: reducedMotion ? 0 : 0.18 },
                        }}
                      />
                    }
                  />
                }
              >
                {ready && activeView === tab.value && (
                  <motion.span
                    aria-hidden="true"
                    layoutId={
                      reducedMotion ? undefined : `${indicatorID}-active`
                    }
                    className="pointer-events-none absolute inset-0 -z-10 rounded-lg bg-builder-surface-hover"
                    initial={reducedMotion ? false : { opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={transition}
                  />
                )}
                <span className="relative">{tab.label}</span>
              </TabsPrimitive.Tab>
              <TooltipContent
                id={`${indicatorID}-${tab.value}-tooltip`}
                role="tooltip"
                side="bottom"
                sideOffset={10}
                className="max-w-64 text-xs leading-5"
              >
                {tab.help}
              </TooltipContent>
            </Tooltip>
          ))}
        </AnimatePresence>
      </TabsPrimitive.List>
    </TooltipProvider>
  );
}
