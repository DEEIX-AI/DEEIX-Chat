import Image from "next/image";

import { cn } from "@/lib/utils";

export function PoweredByDeeix({ className }: { className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 text-[11px] font-medium leading-none text-muted-foreground/70",
        className,
      )}
    >
      <span>Powered by</span>
      <a
        href="https://github.com/DEEIX-AI/DEEIX-Chat"
        target="_blank"
        rel="noopener noreferrer"
        aria-label="DEEIX Chat on GitHub"
        className="inline-flex shrink-0 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:ring-offset-2"
      >
        <Image
          src="/logo.svg"
          alt=""
          aria-hidden="true"
          width={58}
          height={18}
          className="h-3.5 w-auto opacity-65 dark:hidden"
        />
        <Image
          src="/logo-white.svg"
          alt=""
          aria-hidden="true"
          width={58}
          height={18}
          className="hidden h-3.5 w-auto opacity-65 dark:block"
        />
      </a>
    </span>
  );
}
