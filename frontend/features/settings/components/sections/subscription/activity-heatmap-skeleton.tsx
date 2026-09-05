"use client";

// 热力图骨架屏：dynamic 加载期与数据请求期共用同一结构，避免两段骨架之间布局跳变。
export function ActivityHeatmapSkeleton() {
  return (
    <div className="space-y-3 rounded-md bg-muted/35 p-3">
      <div className="flex h-9 items-center justify-between gap-3 px-1">
        <div className="h-4 w-28 rounded-full bg-muted/50" />
        <div className="h-7 w-32 rounded-full bg-muted/50" />
      </div>
      <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => (
          <div key={`activity-heatmap-skeleton-${index}`} className="rounded-md bg-muted/40 p-3">
            <div className="h-3 w-16 rounded-full bg-muted/60" />
            <div className="mt-2 h-4 w-14 rounded-full bg-muted/60" />
          </div>
        ))}
      </div>
      <div className="h-[104px] rounded-md bg-muted/30" />
    </div>
  );
}
