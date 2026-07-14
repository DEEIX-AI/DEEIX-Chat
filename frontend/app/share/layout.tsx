import type { ReactNode } from "react";

import { ShareWorkspace } from "@/features/layouts";

export default function ShareLayout({ children }: { children: ReactNode }) {
  return <ShareWorkspace>{children}</ShareWorkspace>;
}
