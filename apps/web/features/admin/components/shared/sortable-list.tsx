"use client";

import * as React from "react";
import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  type DragEndEvent,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Slot } from "radix-ui";

import { GripVerticalIcon, type GripVerticalIconHandle } from "@/components/ui/grip-vertical";
import { cn } from "@/lib/utils";

export function moveSortableItem<T>(items: T[], index: number, targetIndex: number): T[] {
  if (index < 0 || targetIndex < 0 || index >= items.length || targetIndex >= items.length || index === targetIndex) {
    return items;
  }
  const next = [...items];
  const [item] = next.splice(index, 1);
  next.splice(targetIndex, 0, item);
  return next;
}

export type AdminSortableRenderProps = {
  attributes: ReturnType<typeof useSortable>["attributes"];
  listeners: ReturnType<typeof useSortable>["listeners"];
  isDragging: boolean;
};

export function AdminSortableList({
  children,
  disabled = false,
  items,
  onMove,
}: {
  children: React.ReactNode;
  disabled?: boolean;
  items: string[];
  onMove: (activeID: string, overID: string) => void;
}) {
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 4,
      },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  const handleDragEnd = React.useCallback((event: DragEndEvent) => {
    const { active, over } = event;
    if (disabled || !over || active.id === over.id) {
      return;
    }
    onMove(String(active.id), String(over.id));
  }, [disabled, onMove]);

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      <SortableContext items={items} strategy={verticalListSortingStrategy}>
        {children}
      </SortableContext>
    </DndContext>
  );
}

export function AdminSortableItem({
  asChild = false,
  children,
  className,
  disabled = false,
  id,
}: {
  asChild?: boolean;
  children: (props: AdminSortableRenderProps) => React.ReactNode;
  className?: string;
  disabled?: boolean;
  id: string;
}) {
  const {
    attributes,
    isDragging,
    listeners,
    setNodeRef,
    transform,
    transition,
  } = useSortable({
    id,
    disabled,
  });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  } satisfies React.CSSProperties;
  const Comp = asChild ? Slot.Root : "div";

  return (
    <Comp ref={setNodeRef} style={style} className={className}>
      {children({ attributes, isDragging, listeners })}
    </Comp>
  );
}

export function AdminSortableHandle({
  attributes,
  className,
  disabled = false,
  hidden = false,
  label,
  listeners,
}: {
  attributes: AdminSortableRenderProps["attributes"];
  className?: string;
  disabled?: boolean;
  hidden?: boolean;
  label: string;
  listeners: AdminSortableRenderProps["listeners"];
}) {
  const iconRef = React.useRef<GripVerticalIconHandle>(null);

  if (hidden) {
    return null;
  }

  return (
    <button
      {...attributes}
      {...listeners}
      type="button"
      disabled={disabled}
      aria-label={label}
      title={label}
      className={cn(
        "flex size-5 shrink-0 cursor-grab items-center justify-center rounded-md text-muted-foreground/70 transition-colors hover:bg-accent hover:text-accent-foreground active:cursor-grabbing disabled:cursor-not-allowed disabled:text-muted-foreground/40",
        className,
      )}
      onMouseEnter={() => iconRef.current?.startAnimation()}
      onMouseLeave={() => iconRef.current?.stopAnimation()}
    >
      <GripVerticalIcon ref={iconRef} size={12} className="size-3.5" />
    </button>
  );
}
