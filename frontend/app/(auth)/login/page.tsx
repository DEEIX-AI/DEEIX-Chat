import { Suspense } from "react";

import { LoginRoute } from "@/app/(auth)/login/login-route";

export default function Page() {
  return (
    <Suspense fallback={null}>
      <LoginRoute />
    </Suspense>
  );
}
