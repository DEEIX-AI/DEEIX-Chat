"use client";

import * as React from "react";
import { Check, Copy } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { writeClipboardText } from "@/shared/lib/clipboard";

export type CopyActionMessages = {
  copied: string;
  copiedDescription?: string;
  failed: string;
  failedDescription?: string;
  empty?: string;
  emptyDescription?: string;
};

type CopyActionOptions = {
  messages: CopyActionMessages;
  resetDelayMs?: number;
};

export type CopyActionInput = {
  key?: string;
  copied?: string;
  copiedDescription?: string;
  failed?: string;
  failedDescription?: string;
  empty?: string;
  emptyDescription?: string;
};

export function useCopyAction({ messages, resetDelayMs = 1500 }: CopyActionOptions) {
  const [copied, setCopied] = React.useState(false);
  const [copiedKey, setCopiedKey] = React.useState<string | null>(null);
  const [pending, setPending] = React.useState(false);
  const resetTimerRef = React.useRef<number | null>(null);

  const clearResetTimer = React.useCallback(() => {
    if (resetTimerRef.current) {
      window.clearTimeout(resetTimerRef.current);
      resetTimerRef.current = null;
    }
  }, []);

  const reset = React.useCallback(() => {
    clearResetTimer();
    setCopied(false);
    setCopiedKey(null);
  }, [clearResetTimer]);

  React.useEffect(() => reset, [reset]);

  const copy = React.useCallback(
    async (value: string, overrides: CopyActionInput = {}): Promise<boolean> => {
      const text = value.trim() ? value : "";
      if (!text) {
        setCopied(false);
        setCopiedKey(null);
        const emptyMessage = overrides.empty ?? messages.empty;
        if (emptyMessage) {
          toast.error(emptyMessage, { description: overrides.emptyDescription ?? messages.emptyDescription });
        }
        return false;
      }

      setPending(true);
      clearResetTimer();
      try {
        await writeClipboardText(text);
        setCopied(true);
        setCopiedKey(overrides.key ?? null);
        toast.success(overrides.copied ?? messages.copied, {
          description: overrides.copiedDescription ?? messages.copiedDescription,
        });
        resetTimerRef.current = window.setTimeout(() => {
          setCopied(false);
          setCopiedKey(null);
          resetTimerRef.current = null;
        }, resetDelayMs);
        return true;
      } catch {
        setCopied(false);
        setCopiedKey(null);
        toast.error(overrides.failed ?? messages.failed, {
          description: overrides.failedDescription ?? messages.failedDescription,
        });
        return false;
      } finally {
        setPending(false);
      }
    },
    [clearResetTimer, messages, resetDelayMs],
  );

  const isCopied = React.useCallback((key?: string) => {
    if (key === undefined) {
      return copied;
    }
    return copied && copiedKey === key;
  }, [copied, copiedKey]);

  return {
    copied,
    copy,
    isCopied,
    pending,
    reset,
  };
}

type CopyActionButtonProps = Omit<React.ComponentProps<typeof Button>, "onClick"> & {
  value?: string;
  resolveValue?: () => string | Promise<string>;
  messages: CopyActionMessages;
  copyOptions?: CopyActionInput;
  resetDelayMs?: number;
  iconClassName?: string;
  copyIcon?: React.ReactNode;
  copiedIcon?: React.ReactNode;
  onCopied?: () => void;
  onResolveError?: (error: unknown) => void;
};

export function CopyActionButton({
  value,
  resolveValue,
  messages,
  copyOptions,
  resetDelayMs,
  iconClassName = "size-3.5",
  copyIcon,
  copiedIcon,
  children,
  disabled,
  onCopied,
  onResolveError,
  ...buttonProps
}: CopyActionButtonProps) {
  const { copied, copy, pending } = useCopyAction({ messages, resetDelayMs });
  const [resolving, setResolving] = React.useState(false);

  const handleClick = React.useCallback(async () => {
    setResolving(true);
    try {
      const resolvedValue = resolveValue ? await resolveValue() : (value ?? "");
      const ok = await copy(resolvedValue, copyOptions);
      if (ok) {
        onCopied?.();
      }
    } catch (error) {
      if (onResolveError) {
        onResolveError(error);
      } else {
        toast.error(messages.failed, { description: messages.failedDescription });
      }
    } finally {
      setResolving(false);
    }
  }, [copy, copyOptions, messages, onCopied, onResolveError, resolveValue, value]);

  return (
    <Button
      {...buttonProps}
      disabled={disabled || pending || resolving}
      onClick={() => void handleClick()}
    >
      {copied ? (copiedIcon ?? <Check className={iconClassName} />) : (copyIcon ?? <Copy className={iconClassName} />)}
      {children}
    </Button>
  );
}
