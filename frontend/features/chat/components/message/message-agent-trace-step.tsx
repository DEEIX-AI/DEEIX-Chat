"use client";

import type { ReactNode } from "react";

import { ChevronDown } from "@/components/animate-ui/icons/chevron-down";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { cn } from "@/lib/utils";

const AGENT_TRACE_STEP_VALUE = "details";

export function AgentTraceStep({
  icon,
  title,
  status,
  duration,
  open,
  expandable,
  failed,
  loading,
  children,
  onOpenChange,
}: {
  icon: ReactNode;
  title: string;
  status?: string;
  duration?: string;
  open: boolean;
  expandable: boolean;
  failed?: boolean;
  loading?: boolean;
  children?: ReactNode;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Accordion
      type="single"
      collapsible
      value={open && expandable ? AGENT_TRACE_STEP_VALUE : ""}
      onValueChange={(value) => onOpenChange(value === AGENT_TRACE_STEP_VALUE)}
      className="w-full"
    >
      <AccordionItem value={AGENT_TRACE_STEP_VALUE} className="border-b-0">
        <div className="grid grid-cols-[0.875rem_minmax(0,1fr)] items-start gap-x-3 text-[12px] leading-5 max-sm:gap-x-2">
          <div className="relative flex h-5 items-center justify-center">
            <span className="relative z-10 inline-flex size-3 items-center justify-center bg-background">
              {icon}
            </span>
          </div>
          <AccordionTrigger
            iconPosition="none"
            disabled={!expandable}
            className="min-h-0 min-w-0 items-center justify-start gap-1.5 rounded-none pb-1.5 pt-0 text-left text-[12px] leading-5 no-underline hover:no-underline"
          >
            <span
              className={cn(
                "shrink-0 font-medium text-muted-foreground/82 transition-colors group-hover/agent-trace-step:text-foreground",
                failed && "text-destructive/85 group-hover/agent-trace-step:text-destructive",
                loading && "shimmer",
              )}
            >
              {title}
            </span>
            {status ? <span className="shrink-0 font-normal text-muted-foreground/58">{status}</span> : null}
            {duration ? (
              <span className="shrink-0 font-normal tabular-nums text-muted-foreground/58">{duration}</span>
            ) : null}
            {expandable ? (
              <ChevronDown
                className={cn(
                  "size-3 shrink-0 text-muted-foreground/58 transition-transform duration-200",
                  !open && "-rotate-90",
                )}
              />
            ) : null}
          </AccordionTrigger>
        </div>
        {expandable ? (
          <AccordionContent className="px-0 pb-2 pl-[calc(0.875rem+0.75rem)] pt-0 duration-[350ms] ease-in-out max-sm:pl-[calc(0.875rem+0.5rem)]">
            {children}
          </AccordionContent>
        ) : null}
      </AccordionItem>
    </Accordion>
  );
}
