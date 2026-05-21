import { Suspense } from "react";

import { AppChatArea } from "@/features/chat/components/app-chat-area";

export default function Page() {
  return (
    <Suspense fallback={null}>
      <AppChatArea />
    </Suspense>
  );
}
