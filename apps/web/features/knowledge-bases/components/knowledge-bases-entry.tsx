"use client";

import { useRouter } from "next/navigation";
import * as React from "react";

import { AppKnowledgeBases } from "@/features/knowledge-bases/components/app-knowledge-bases";
import { useFeaturePolicy } from "@/shared/hooks/use-feature-policy";

// Route entry for /knowledge-bases: the workspace only mounts while the server
// policy enables knowledge bases, otherwise the user is sent home.
export function KnowledgeBasesEntry() {
  const router = useRouter();
  const { knowledgeBaseEnabled, loaded } = useFeaturePolicy();

  React.useEffect(() => {
    if (loaded && !knowledgeBaseEnabled) router.replace("/");
  }, [knowledgeBaseEnabled, loaded, router]);

  if (!loaded || !knowledgeBaseEnabled) return null;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 overflow-hidden md:-mx-4 md:-mb-4">
      <AppKnowledgeBases />
    </div>
  );
}
