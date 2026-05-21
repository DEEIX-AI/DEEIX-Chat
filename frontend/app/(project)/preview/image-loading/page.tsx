"use client";

import { useTranslations } from "next-intl";

import { AssistantImageGenerationSkeleton } from "@/features/chat/components/message/message-bot";

export default function Page() {
  const t = useTranslations("chat.submit.mediaStatus");
  const label = t("running");

  return (
    <main className="flex h-full min-h-0 w-full items-start justify-center overflow-auto bg-background px-6 py-12">
      <div className="w-full max-w-4xl">
        <div className="mb-8">
          <h1 className="text-xl font-medium tracking-normal text-foreground">Image loading preview</h1>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            Preview loading ratios for image generation.
          </p>
        </div>
        <div className="grid gap-8 md:grid-cols-[minmax(0,1fr)_18rem]">
          <div className="space-y-8">
            <AssistantImageGenerationSkeleton label={label} aspectRatio="wide" />
            <AssistantImageGenerationSkeleton label={label} aspectRatio="square" />
          </div>
          <AssistantImageGenerationSkeleton label={label} aspectRatio="portrait" />
        </div>
      </div>
    </main>
  );
}
