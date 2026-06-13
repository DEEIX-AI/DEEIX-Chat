"use client";

import { useSearchParams } from "next/navigation";

import { LoginPage } from "@/features/auth/components/login-page";
import { normalizeAuthNextPath } from "@/shared/auth/local-path";

export function LoginRoute() {
  const searchParams = useSearchParams();
  return <LoginPage nextPath={normalizeAuthNextPath(searchParams.get("next"), "")} />;
}
