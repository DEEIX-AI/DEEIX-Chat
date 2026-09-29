import type { ReactNode } from "react";

import { ShareWorkspace } from "@/features/shell";

export default function ShareLayout({ children }: { children: ReactNode }) {
  return <ShareWorkspace>{children}</ShareWorkspace>;
}
