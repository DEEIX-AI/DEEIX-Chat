import type { ReactNode } from "react";

import { AdminSidebar } from "@/features/admin/components/admin-sidebar";
import { CustomBrandAttribution } from "@/shared/components/powered-by-deeix";

export function AdminShell({
  children,
  basePath = "/admin",
}: {
  children: ReactNode;
  basePath?: string;
}) {
  return (
    <div className="h-full min-h-0 w-full flex-1 overflow-x-hidden overflow-y-auto bg-background [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      <div className="mx-auto flex min-h-full w-full max-w-[1230px] flex-col gap-4 px-3 py-4 md:px-6 xl:flex-row xl:gap-8 xl:px-0 xl:py-6">
        <AdminSidebar basePath={basePath} />
        <main className="min-w-0 flex-1">
          <div className="mx-auto w-full min-w-0 max-w-[1080px] xl:pt-20">
            {children}
          </div>
        </main>
      </div>

      <CustomBrandAttribution className="fixed bottom-4 right-4" />
    </div>
  );
}
