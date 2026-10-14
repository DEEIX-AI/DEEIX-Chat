"use client";

import type * as React from "react";

import { AppLogo } from "@/shared/components/app-logo";

export const MODELS_SETTINGS_PATH = "/settings/models";
export const PRIMARY_BUTTON_CLASS = "h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90";
export const CANCEL_BUTTON_CLASS = "h-9 w-full rounded-md text-sm font-normal text-muted-foreground shadow-none hover:bg-muted/60 hover:text-foreground";

/** Centered card for the import screens. */
export function ImportFrame({
  title,
  description,
  wide = false,
  children,
}: {
  title?: string;
  description?: string;
  /** Wider layout for multi-provider lists. */
  wide?: boolean;
  children: React.ReactNode;
}) {
  return (
    <main className="flex min-h-screen animate-in items-center justify-center px-4 py-8 text-foreground fade-in-0 duration-300">
      <div className={wide ? "w-full max-w-[460px]" : "w-full max-w-[360px]"}>
        <AppLogo width={32} height={32} priority className="mx-auto h-9 w-auto" />
        <div className="space-y-5 px-2 pt-7">
          {title ? (
            <div className="space-y-1.5 text-center">
              <h1 className="text-base font-semibold">{title}</h1>
              {description ? <p className="text-xs leading-relaxed text-muted-foreground">{description}</p> : null}
            </div>
          ) : null}
          {children}
        </div>
      </div>
    </main>
  );
}
