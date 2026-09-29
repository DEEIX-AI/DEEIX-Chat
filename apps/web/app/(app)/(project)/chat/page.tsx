import { Suspense } from "react";

import { AppChatArea } from "@/features/chat";

export default function Page() {
  return (
    <Suspense fallback={null}>
      <AppChatArea />
    </Suspense>
  );
}
