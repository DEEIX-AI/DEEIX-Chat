"use client";

import * as React from "react";

import { cn } from "@/lib/utils";

export function TypographyH1({ className, ...props }: React.ComponentPropsWithoutRef<"h1">) {
  return <h1 className={cn("scroll-m-20 text-4xl font-extrabold tracking-tight text-balance", className)} {...props} />;
}

export function TypographyH2({ className, ...props }: React.ComponentPropsWithoutRef<"h2">) {
  return <h2 className={cn("scroll-m-20 border-b pb-2 text-3xl font-semibold tracking-tight first:mt-0", className)} {...props} />;
}

export function TypographyH3({ className, ...props }: React.ComponentPropsWithoutRef<"h3">) {
  return <h3 className={cn("scroll-m-20 text-2xl font-semibold tracking-tight", className)} {...props} />;
}

export function TypographyH4({ className, ...props }: React.ComponentPropsWithoutRef<"h4">) {
  return <h4 className={cn("scroll-m-20 text-xl font-semibold tracking-tight", className)} {...props} />;
}

export function TypographyH5({ className, ...props }: React.ComponentPropsWithoutRef<"h5">) {
  return <h5 className={cn("scroll-m-20 text-lg font-semibold tracking-tight", className)} {...props} />;
}

export function TypographyH6({ className, ...props }: React.ComponentPropsWithoutRef<"h6">) {
  return <h6 className={cn("scroll-m-20 text-base font-semibold tracking-tight", className)} {...props} />;
}

export function TypographyP({ className, ...props }: React.ComponentPropsWithoutRef<"p">) {
  return <p className={cn("leading-7 whitespace-pre-wrap break-words [&:not(:first-child)]:mt-6", className)} {...props} />;
}

export function TypographyBlockquote({ className, ...props }: React.ComponentPropsWithoutRef<"blockquote">) {
  return <blockquote className={cn("mt-6 border-l-2 pl-6 italic text-foreground/85", className)} {...props} />;
}

export function TypographyList({ className, ...props }: React.ComponentPropsWithoutRef<"ul">) {
  return <ul className={cn("my-6 ml-6 list-disc [&>li]:mt-2", className)} {...props} />;
}

export function TypographyOrderedList({ className, ...props }: React.ComponentPropsWithoutRef<"ol">) {
  return <ol className={cn("my-6 ml-6 list-decimal [&>li]:mt-2", className)} {...props} />;
}

export function TypographyInlineCode({ className, ...props }: React.ComponentPropsWithoutRef<"code">) {
  return (
    <code
      className={cn("relative rounded bg-muted px-[0.3rem] py-[0.2rem] font-mono text-sm font-semibold", className)}
      {...props}
    />
  );
}

export function TypographyLead({ className, ...props }: React.ComponentPropsWithoutRef<"p">) {
  return <p className={cn("text-xl text-muted-foreground", className)} {...props} />;
}

export function TypographyLarge({ className, ...props }: React.ComponentPropsWithoutRef<"div">) {
  return <div className={cn("text-lg font-semibold", className)} {...props} />;
}

export function TypographySmall({ className, ...props }: React.ComponentPropsWithoutRef<"small">) {
  return <small className={cn("text-sm leading-none font-medium", className)} {...props} />;
}

export function TypographyMuted({ className, ...props }: React.ComponentPropsWithoutRef<"p">) {
  return <p className={cn("text-sm text-muted-foreground", className)} {...props} />;
}

export function TypographyHr({ className, ...props }: React.ComponentPropsWithoutRef<"hr">) {
  return <hr className={cn("my-6 border-border", className)} {...props} />;
}

export function TypographyTable({
  className,
  ...props
}: React.ComponentPropsWithoutRef<"table">) {
  return (
    <div className="my-6 w-full overflow-x-auto">
      <table className={cn("w-full", className)} {...props} />
    </div>
  );
}

export function TypographyCodeBlock({
  className,
  language,
  children,
  ...props
}: React.ComponentPropsWithoutRef<"pre"> & {
  language?: string;
}) {
  return (
    <div className="my-6 overflow-hidden rounded-2xl border border-border/70 bg-card/75">
      {language ? (
        <div className="border-b border-border/70 px-4 py-2 font-mono text-xs text-muted-foreground">
          {language}
        </div>
      ) : null}
      <pre
        className={cn("overflow-x-auto px-4 py-4 font-mono text-[13px] leading-6 text-foreground", className)}
        {...props}
      >
        {children}
      </pre>
    </div>
  );
}
