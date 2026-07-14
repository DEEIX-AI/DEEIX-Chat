"use client";

import * as React from "react";
import { Check, Folder } from "lucide-react";

import {
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

export type ConversationProjectSubmenuProject = {
  publicID: string;
  name: string;
};

type ConversationProjectSubmenuProps = {
  label: string;
  unassignedLabel: string;
  currentProjectID?: string;
  projects: ConversationProjectSubmenuProject[];
  onSelect: (projectID: string) => void | Promise<void>;
};

export function ConversationProjectSubmenu({
  label,
  unassignedLabel,
  currentProjectID,
  projects,
  onSelect,
}: ConversationProjectSubmenuProps) {
  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger>
        <Folder className="size-3.5 text-current" strokeWidth={1.6} />
        {label}
      </DropdownMenuSubTrigger>
      <DropdownMenuSubContent className="max-h-[min(18rem,var(--radix-dropdown-menu-content-available-height))] min-w-44 overflow-y-auto p-1.5">
        <DropdownMenuItem
          className="justify-between gap-6"
          onSelect={() => {
            void onSelect("");
          }}
        >
          <span className="flex min-w-0 items-center gap-2">
            <Folder className="size-3.5 shrink-0 text-current" strokeWidth={1.6} />
            <span className="min-w-0 truncate">{unassignedLabel}</span>
          </span>
          <Check
            aria-hidden="true"
            className={cn("size-3.5 shrink-0", !currentProjectID ? "text-foreground" : "opacity-0")}
            strokeWidth={1.7}
          />
        </DropdownMenuItem>
        {projects.map((project) => {
          const selected = currentProjectID === project.publicID;

          return (
            <DropdownMenuItem
              key={project.publicID}
              className="justify-between gap-6"
              onSelect={() => {
                void onSelect(project.publicID);
              }}
            >
              <span className="flex min-w-0 items-center gap-2">
                <Folder className="size-3.5 shrink-0 text-current" strokeWidth={1.6} />
                <span className="min-w-0 truncate">{project.name}</span>
              </span>
              <Check
                aria-hidden="true"
                className={cn("size-3.5 shrink-0", selected ? "text-foreground" : "opacity-0")}
                strokeWidth={1.7}
              />
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  );
}
