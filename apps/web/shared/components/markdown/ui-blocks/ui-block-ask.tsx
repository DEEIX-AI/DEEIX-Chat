"use client";

import { RotateCcw, Send } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

// Longer than the model's suggestion (capped at BLOCK_LIMITS.askLength): what
// the user types is theirs to send.
const ASK_MAX_LENGTH = 2000;
const ASK_MAX_HEIGHT = 160;

// A follow-up the model suggests, as an editable message: the text field holds
// exactly what will be sent, and nothing is sent until the user presses send
// (or Enter). Until the user edits it, it follows the block's state (the
// suggestion is a template); once edited it keeps the user's words, and a
// button brings the suggestion back. Shared by `ask` buttons in blocks and by
// inline illustrations, so every follow-up looks and behaves the same.
export function UIBlockAsk({ suggestion, busy, send }: { suggestion: string; busy: boolean; send?: (text: string) => void }) {
  const t = useTranslations("chat.markdown.uiBlock");
  const [draft, setDraft] = React.useState(suggestion);
  const [edited, setEdited] = React.useState(false);
  const [lastSuggestion, setLastSuggestion] = React.useState(suggestion);
  if (suggestion !== lastSuggestion) {
    setLastSuggestion(suggestion);
    if (!edited) {
      setDraft(suggestion);
    }
  }
  const fieldRef = React.useRef<HTMLTextAreaElement | null>(null);
  // Grows with its text up to a few lines, then scrolls.
  React.useLayoutEffect(() => {
    const field = fieldRef.current;
    if (!field) {
      return;
    }
    field.style.height = "auto";
    field.style.height = `${Math.min(field.scrollHeight, ASK_MAX_HEIGHT)}px`;
  }, [draft]);

  const unavailable = !send;
  const text = draft.trim();
  const canSend = Boolean(send) && !busy && text !== "";
  const hint = unavailable ? t("askUnavailable") : busy ? t("askBusy") : t("askHint");
  const submit = () => {
    if (!canSend) {
      return;
    }
    send?.(text);
    setEdited(false);
    setDraft(suggestion);
  };
  return (
    <div className="flex w-full min-w-0 items-end gap-1.5 rounded-lg border-[0.5px] border-border bg-muted/30 py-1.5 pl-3 pr-1.5 transition-colors focus-within:border-primary/50 focus-within:bg-background">
      <textarea
        ref={fieldRef}
        rows={1}
        value={draft}
        maxLength={ASK_MAX_LENGTH}
        readOnly={unavailable}
        aria-label={t("askLabel")}
        placeholder={t("askPlaceholder")}
        onChange={(event) => {
          setDraft(event.target.value);
          setEdited(true);
        }}
        onKeyDown={(event) => {
          // Enter sends, Shift+Enter breaks the line; never while composing (IME).
          if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing && event.keyCode !== 229) {
            event.preventDefault();
            submit();
          }
        }}
        className="min-w-0 flex-1 resize-none bg-transparent py-1 text-sm leading-6 text-foreground/90 outline-none placeholder:text-muted-foreground read-only:cursor-default"
      />
      {edited && draft !== suggestion ? (
        <button
          type="button"
          onClick={() => {
            setEdited(false);
            setDraft(suggestion);
          }}
          aria-label={t("askRestore")}
          title={t("askRestore")}
          className="flex size-7 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <RotateCcw className="size-3.5" strokeWidth={1.8} />
        </button>
      ) : null}
      <button
        type="button"
        onClick={submit}
        disabled={!canSend}
        aria-label={t("askSend")}
        title={hint}
        className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:bg-muted disabled:text-muted-foreground"
      >
        <Send className="size-3.5" strokeWidth={1.8} />
      </button>
    </div>
  );
}
