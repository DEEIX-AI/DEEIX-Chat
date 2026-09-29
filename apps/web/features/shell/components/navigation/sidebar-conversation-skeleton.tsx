import { SidebarMenu, SidebarMenuSkeleton } from "@/components/ui/sidebar";

export function SidebarConversationSkeleton({
  count,
  widths,
  prefix,
}: {
  count: number;
  widths: readonly string[];
  prefix: string;
}) {
  return (
    <SidebarMenu aria-hidden="true">
      {Array.from({ length: count }).map((_, index) => (
        <li key={`${prefix}-skeleton-${index}`}>
          <SidebarMenuSkeleton textWidth={widths[index % widths.length]} />
        </li>
      ))}
    </SidebarMenu>
  );
}
