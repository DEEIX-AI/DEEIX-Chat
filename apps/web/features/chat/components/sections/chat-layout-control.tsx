"use client";

import * as React from "react";
import { AlignCenter, AlignLeft, Columns3 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import {
  CHAT_CONTENT_ALIGN_OPTIONS,
  CHAT_CONTENT_WIDTH_OPTIONS,
  parseChatContentAlign,
  parseChatContentWidth,
  updateUserSettings,
  useUserSettings,
} from "@/entities/user-settings";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export function ChatLayoutControl({ showLabel = false }: { showLabel?: boolean }) {
  const { accessToken } = useAuthSession();
  const { settings } = useUserSettings();
  const t = useTranslations("settings.chatPage.display");
  const [saving, setSaving] = React.useState(false);
  const width = parseChatContentWidth(settings["chat.content_width"]);
  const align = parseChatContentAlign(settings["chat.content_align"]);

  if (!accessToken) {
    return null;
  }

  const apply = async (key: "chat.content_width" | "chat.content_align", value: string) => {
    const token = accessToken.trim() || (await resolveAccessToken());
    if (!token) {
      return;
    }
    setSaving(true);
    try {
      await updateUserSettings(token, { [key]: value });
    } catch (error) {
      toast.error(String(error));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={t("widthTitle")}
          title={t("widthTitle")}
          className={
            showLabel
              ? "inline-flex h-8 items-center gap-1.5 rounded-full border border-border/60 bg-background/80 px-3 text-[11px] font-medium text-muted-foreground shadow-sm transition-colors hover:border-primary/40 hover:text-foreground"
              : "inline-flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
          }
        >
          <Columns3 className="size-3.5" />
          {showLabel ? <span>{t("layoutButton")}</span> : null}
        </button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 p-3">
        <div className="space-y-3">
          <div>
            <p className="mb-1.5 text-xs font-medium text-muted-foreground">{t("widthTitle")}</p>
            <p className="mb-2 text-[11px] leading-4 text-muted-foreground/80">{t("widthHint")}</p>
            <div className="grid grid-cols-4 gap-1.5">
              {CHAT_CONTENT_WIDTH_OPTIONS.map((item) => (
                <button
                  key={item.value}
                  type="button"
                  disabled={saving}
                  className={cn(
                    "rounded-md border px-1 py-1.5 text-[11px] transition-colors",
                    width === item.value
                      ? "border-primary/60 bg-primary/10 text-primary"
                      : "border-border/60 text-muted-foreground hover:text-foreground",
                  )}
                  onClick={() => void apply("chat.content_width", item.value)}
                >
                  {t(`width.${item.value}`)}
                </button>
              ))}
            </div>
          </div>

          <div>
            <p className="mb-1.5 text-xs font-medium text-muted-foreground">{t("alignTitle")}</p>
            <div className="grid grid-cols-2 gap-1.5">
              {CHAT_CONTENT_ALIGN_OPTIONS.map((item) => (
                <button
                  key={item.value}
                  type="button"
                  disabled={saving}
                  className={cn(
                    "inline-flex items-center justify-center gap-1.5 rounded-md border px-2 py-1.5 text-[11px] transition-colors",
                    align === item.value
                      ? "border-primary/60 bg-primary/10 text-primary"
                      : "border-border/60 text-muted-foreground hover:text-foreground",
                  )}
                  onClick={() => void apply("chat.content_align", item.value)}
                >
                  {item.value === "left" ? <AlignLeft className="size-3.5" /> : <AlignCenter className="size-3.5" />}
                  {t(`align.${item.value}`)}
                </button>
              ))}
            </div>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}