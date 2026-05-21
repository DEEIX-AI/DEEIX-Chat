import type { ReactNode } from "react";

import { AdminSidebar } from "@/features/admin/components/admin-sidebar";
import type { AdminSection } from "@/features/admin/model/admin-sections";

export function AdminShell({
  activeSection,
  children,
  basePath = "/admin",
}: {
  activeSection: AdminSection;
  children: ReactNode;
  basePath?: string;
}) {
  return (
    <div className="flex h-full min-h-0 w-full flex-1 overflow-hidden bg-background">
      <div className="mx-auto flex h-full min-h-0 w-full max-w-[1230px] flex-col gap-4 overflow-hidden px-3 py-4 md:px-6 xl:flex-row xl:gap-8 xl:px-0 xl:py-6">
        <AdminSidebar activeSection={activeSection} basePath={basePath} />
        <main className="min-h-0 min-w-0 flex-1 overflow-y-auto [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden">
          <div className="mx-auto w-full max-w-[1080px] xl:pt-20">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
