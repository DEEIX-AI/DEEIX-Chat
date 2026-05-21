import { Bot } from "lucide-react";

import { cn } from "@/lib/utils";

export function LobeHubIcon({
  iconUrl,
  label,
  size = 16,
  className,
  fallbackClassName,
}: {
  iconUrl?: string | null;
  label: string;
  size?: number;
  className?: string;
  fallbackClassName?: string;
}) {
  const dimension = `${size}px`;
  return (
    <span className={cn("inline-flex shrink-0 items-center justify-center", className)} style={{ width: dimension, height: dimension }}>
      {iconUrl ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          alt=""
          aria-hidden="true"
          className="block size-full object-contain dark:invert"
          decoding="async"
          loading="lazy"
          src={iconUrl}
        />
      ) : (
        <Bot className={cn("size-full text-muted-foreground", fallbackClassName)} />
      )}
      <span className="sr-only">{label}</span>
    </span>
  );
}
