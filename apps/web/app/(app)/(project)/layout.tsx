import type { ReactNode } from "react";

import { ProjectWorkspace } from "@/features/layouts";

export default function ProjectRouteLayout({ children }: { children: ReactNode }) {
  return <ProjectWorkspace>{children}</ProjectWorkspace>;
}
