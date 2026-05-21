"use client";

import * as React from "react";
import { Plus, Search } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type RecentHeaderProps = {
  query: string;
  onQueryChange: (value: string) => void;
  onCreateConversation: () => void | Promise<void>;
};

export function RecentHeader({ query, onQueryChange, onCreateConversation }: RecentHeaderProps) {
  const t = useTranslations("recent");

  return (
    <div className="ml-0 md:ml-13 md:w-[calc(100%-3.25rem)]">
      <div className="flex items-start justify-between gap-4">
        <h1 className="text-xl font-semibold tracking-[-0.03em] text-foreground md:text-2xl">{t("allConversations")}</h1>
        <Button size="sm" variant="default" className="shrink-0" onClick={() => void onCreateConversation()}>
          <Plus className="size-4" />
          {t("newChat")}
        </Button>
      </div>

      <div className="relative mt-6 md:mt-10">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={t("searchPlaceholder")}
          className="rounded-xl bg-background pl-9"
        />
      </div>
    </div>
  );
}
