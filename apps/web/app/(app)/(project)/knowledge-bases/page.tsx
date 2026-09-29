import { Suspense } from "react";

import { KnowledgeBasesEntry } from "@/features/knowledge-bases";

export default function Page() {
  return (
    <Suspense fallback={null}>
      <KnowledgeBasesEntry />
    </Suspense>
  );
}
