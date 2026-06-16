"use client";

import { useTranslations } from "next-intl";

import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import {
  type ChatFontOption,
  type ChatFontWeightOption,
} from "@/features/settings/utils/chat-font";
import type {
  ChatFontPreview,
  ChatFontWeightPreview,
} from "@/features/settings/types/settings";
import { cn } from "@/lib/utils";
import {
  CHAT_CONTENT_WIDTH_OPTIONS,
  type ChatContentWidth,
  type ChatContentWidthOption,
} from "@/shared/model/chat-content-width";

const CHAT_FONT_OPTIONS: ChatFontPreview[] = [
  { label: "Default", value: "default", fontFamily: "var(--font-sans)", sampleText: "Aa" },
  { label: "Serif", value: "songti", fontFamily: "var(--font-songti)", sampleText: "Aa" },
  { label: "Sans", value: "heiti", fontFamily: "var(--font-heiti)", sampleText: "Aa" },
  { label: "Mono", value: "mono", fontFamily: "var(--font-mono)", sampleText: "Aa" },
];

const CHAT_FONT_WEIGHT_OPTIONS: ChatFontWeightPreview[] = [
  { label: "Regular", value: "regular", fontWeight: 400, sampleText: "Aa" },
  { label: "Medium", value: "medium", fontWeight: 500, sampleText: "Aa" },
  { label: "Semibold", value: "semibold", fontWeight: 600, sampleText: "Aa" },
  { label: "Bold", value: "bold", fontWeight: 700, sampleText: "Aa" },
];

function ChatContentWidthPreviewCard({
  item,
  active,
  disabled,
  onSelect,
}: {
  item: ChatContentWidthOption & { label: string };
  active: boolean;
  disabled?: boolean;
  onSelect: (value: ChatContentWidth) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(item.value)}
      className="group text-left disabled:pointer-events-none disabled:opacity-60"
      aria-pressed={active}
      disabled={disabled}
    >
      <div
        className={cn(
          "flex h-24 w-full flex-col items-center justify-center gap-2 rounded-xl border bg-background px-2 transition-all duration-200 hover:scale-102 hover:border-primary/60",
          active ? "border-primary/60" : "border-border/50",
        )}
      >
        <span className={cn("h-2 rounded-full bg-foreground/75", item.previewScaleClassName)} aria-hidden="true" />
        <span className="truncate text-center text-sm font-medium leading-none text-foreground/90">
          {item.label} - {item.width}px
        </span>
      </div>
    </button>
  );
}

function ChatFontPreviewCard({
  item,
  active,
  onSelect,
}: {
  item: ChatFontPreview;
  active: boolean;
  onSelect: (value: ChatFontOption) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(item.value)}
      className="group text-left"
      aria-pressed={active}
    >
      <div
        className={cn(
          "flex h-24 w-full items-center justify-center rounded-xl border bg-background px-1 transition-all duration-200 hover:scale-102 hover:border-primary/60",
          active ? "border-primary/60" : "border-border/50",
        )}
      >
        <span
          className="truncate text-center text-base leading-none text-foreground/90"
          style={{ fontFamily: item.fontFamily }}
        >
          {item.label} {item.sampleText}
        </span>
      </div>
    </button>
  );
}

function ChatFontWeightPreviewCard({
  item,
  active,
  onSelect,
}: {
  item: ChatFontWeightPreview;
  active: boolean;
  onSelect: (value: ChatFontWeightOption) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(item.value)}
      className="group text-left"
      aria-pressed={active}
    >
      <div
        className={cn(
          "flex h-24 w-full items-center justify-center rounded-xl border bg-background px-1 transition-all duration-200 hover:scale-102 hover:border-primary/60",
          active ? "border-primary/60" : "border-border/50",
        )}
      >
        <span
          className="truncate text-center text-base leading-none text-foreground/90"
          style={{ fontFamily: "var(--font-chat)", fontWeight: item.fontWeight }}
        >
          {item.label} {item.sampleText}
        </span>
      </div>
    </button>
  );
}

export function ChatDisplayAppearance({
  contentWidth,
  chatFont,
  chatFontWeight,
  onContentWidthChange,
  onChatFontChange,
  onChatFontWeightChange,
  disabled,
}: {
  contentWidth: ChatContentWidth;
  chatFont: ChatFontOption;
  chatFontWeight: ChatFontWeightOption;
  onContentWidthChange: (value: ChatContentWidth) => void;
  onChatFontChange: (value: ChatFontOption) => void;
  onChatFontWeightChange: (value: ChatFontWeightOption) => void;
  disabled?: boolean;
}) {
  const t = useTranslations("settings.chatPage.display");

  return (
    <FieldGroup className="gap-3 md:gap-4">
      <Field>
        <FieldLabel>{t("widthTitle")}</FieldLabel>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:gap-3 xl:gap-4">
          {CHAT_CONTENT_WIDTH_OPTIONS.map((item) => (
            <ChatContentWidthPreviewCard
              key={item.value}
              item={{ ...item, label: t(`width.${item.value}`) }}
              active={contentWidth === item.value}
              onSelect={onContentWidthChange}
              disabled={disabled}
            />
          ))}
        </div>
      </Field>

      <Field>
        <FieldLabel>{t("chatFont")}</FieldLabel>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:gap-3 xl:gap-4">
          {CHAT_FONT_OPTIONS.map((item) => (
            <ChatFontPreviewCard
              key={item.value}
              item={{ ...item, label: t(`font.${item.value}`) }}
              active={chatFont === item.value}
              onSelect={onChatFontChange}
            />
          ))}
        </div>
      </Field>

      <Field>
        <FieldLabel>{t("chatFontWeight")}</FieldLabel>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 md:gap-3 xl:gap-4">
          {CHAT_FONT_WEIGHT_OPTIONS.map((item) => (
            <ChatFontWeightPreviewCard
              key={item.value}
              item={{ ...item, label: t(`fontWeight.${item.value}`) }}
              active={chatFontWeight === item.value}
              onSelect={onChatFontWeightChange}
            />
          ))}
        </div>
      </Field>
    </FieldGroup>
  );
}
