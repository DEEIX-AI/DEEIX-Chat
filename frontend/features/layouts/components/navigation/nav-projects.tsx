"use client";

import * as React from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  type DragEndEvent,
  type DragStartEvent,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { AnimatePresence, motion, type Transition } from "motion/react";
import { ChevronDown, PencilLine, Star, StarOff, Trash } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Ellipsis } from "@/components/animate-ui/icons/ellipsis";
import { FolderArchiveIcon } from "@/components/ui/folder-archive";
import { FolderOpenIcon } from "@/components/ui/folder-open";
import { PlusIcon } from "@/components/ui/plus";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Collapsible } from "@/components/ui/collapsible";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { GripVerticalIcon, type GripVerticalIconHandle } from "@/components/ui/grip-vertical";
import { Spinner } from "@/components/ui/spinner";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  SidebarGroup,
  SidebarGroupAction,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar";
import {
  ConversationShareDialog,
  sharePatchFromDTO,
  useConversationExport,
  useSidebarConversations,
} from "@/entities/conversation";
import { useChatSession } from "@/features/chat";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { DeleteFilesOption } from "@/shared/components/delete-files-option";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { useSettingsChatPreferences } from "@/features/settings";
import { useLayoutActiveConversation } from "@/features/layouts/hooks/use-layout-active-conversation";
import { useLayoutProjectConversations } from "@/features/layouts/hooks/use-layout-project-conversations";
import { useSidebarConversationNavigation } from "@/features/layouts/hooks/use-sidebar-conversation-navigation";
import { SidebarConversationItem } from "@/features/layouts/components/navigation/sidebar-conversation-item";
import type {
  SidebarConversationDeleteTarget,
  SidebarConversationRenameTarget,
} from "@/features/layouts/types/navigation";
import { useStoredBoolean } from "@/shared/hooks/use-stored-boolean";
import { cn } from "@/lib/utils";

type ProjectDraft = {
  publicID?: string;
  name: string;
  systemPrompt: string;
};

type ProjectActionTarget = {
  publicID?: string;
  name: string;
};

const PROJECT_TREE_ACCORDION_TRANSITION: Transition = {
  duration: 0.26,
  ease: [0.22, 1, 0.36, 1],
};
const PROJECT_DIALOG_LAYOUT_TRANSITION = {
  layout: {
    duration: 0.22,
    ease: [0.16, 1, 0.3, 1] as const,
  },
};
const PROJECT_TREE_ACCORDION_MASK_STYLE = {
  maskImage: "linear-gradient(black var(--mask-stop), transparent var(--mask-stop))",
  WebkitMaskImage: "linear-gradient(black var(--mask-stop), transparent var(--mask-stop))",
  overflow: "hidden",
} satisfies React.CSSProperties;
const PROJECTS_OPEN_STORAGE_KEY = "deeix.sidebar.projects.open";

type ProjectFolderIconHandle = {
  startAnimation: () => void;
  stopAnimation: () => void;
};

function ProjectGroupHeader({
  title,
  createLabel,
  contentID,
  open,
  onCreate,
  onOpenChange,
  toggleLabel,
}: {
  title: string;
  createLabel: string;
  contentID: string;
  open: boolean;
  onCreate: () => void;
  onOpenChange: (open: boolean) => void;
  toggleLabel: string;
}) {
  const [createHovered, setCreateHovered] = React.useState(false);

  return (
    <div className="group/project-create flex h-8 items-center gap-1">
      <SidebarGroupLabel
        asChild
        className="w-fit max-w-full self-start cursor-pointer gap-1 pr-1 transition-[color,margin,opacity] hover:text-sidebar-foreground"
      >
        <Button
          type="button"
          variant="ghost"
          className="h-8 gap-1 py-0 pl-2 pr-1 text-xs hover:bg-transparent has-[>svg]:pl-2 has-[>svg]:pr-1 dark:hover:bg-transparent"
          aria-controls={contentID}
          aria-expanded={open}
          aria-label={toggleLabel}
          onClick={() => onOpenChange(!open)}
        >
          <span className="min-w-0 truncate text-left">{title}</span>
          <ChevronDown
            aria-hidden
            className={cn(
              "!size-3 stroke-1.5 transition-transform duration-200",
              !open && "-rotate-90",
            )}
          />
        </Button>
      </SidebarGroupLabel>
      <SidebarGroupAction
        type="button"
        aria-label={createLabel}
        className="relative top-auto right-auto ml-auto size-7 shrink-0 text-sidebar-foreground/45 opacity-100 transition-[color,opacity,transform] duration-150 after:pointer-events-none hover:bg-transparent hover:text-sidebar-foreground dark:hover:bg-transparent md:opacity-0 md:group-hover/project-create:opacity-100 md:group-has-[:focus-visible]/project-create:opacity-100"
        onMouseEnter={() => setCreateHovered(true)}
        onMouseLeave={() => setCreateHovered(false)}
        onClick={onCreate}
      >
        <PlusIcon aria-hidden size={14} strokeWidth={1.8} animate={createHovered ? "default" : undefined} />
      </SidebarGroupAction>
    </div>
  );
}

function ProjectTreeButton({
  active,
  actionPaddingClassName = "pr-24",
  contentID,
  expanded,
  name,
  onHoverChange,
  onToggleExpanded,
}: {
  active: boolean;
  actionPaddingClassName?: string;
  contentID: string;
  expanded: boolean;
  name: string;
  onHoverChange?: (hovered: boolean) => void;
  onToggleExpanded: () => void;
}) {
  const iconRef = React.useRef<ProjectFolderIconHandle>(null);

  return (
    <Button
      type="button"
      variant="ghost"
      className={cn(
        "flex h-8 w-full min-w-0 items-center gap-0 rounded-md px-0 text-sm font-normal outline-hidden ring-sidebar-ring transition-colors focus-visible:ring-2",
        active
          ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
          : "text-sidebar-foreground group-hover/project-row:bg-sidebar-accent group-hover/project-row:text-sidebar-accent-foreground",
        actionPaddingClassName,
      )}
      aria-controls={contentID}
      aria-expanded={expanded}
      aria-label={name}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onToggleExpanded();
      }}
      onMouseEnter={() => {
        onHoverChange?.(true);
        iconRef.current?.startAnimation();
      }}
      onMouseLeave={() => {
        onHoverChange?.(false);
        iconRef.current?.stopAnimation();
      }}
    >
      <span className="flex h-8 w-8 shrink-0 items-center justify-center">
        {expanded ? (
          <FolderOpenIcon
            aria-hidden
            ref={iconRef}
            strokeWidth={1.5}
            className="flex size-4 shrink-0 items-center justify-center text-current"
          />
        ) : (
          <FolderArchiveIcon
            aria-hidden
            ref={iconRef}
            strokeWidth={1.5}
            className="flex size-4 shrink-0 items-center justify-center text-current"
          />
        )}
      </span>
      <span className="ml-1 min-w-0 flex-1 truncate text-left">{name}</span>
    </Button>
  );
}

type ProjectInlineActionProps = React.ComponentPropsWithoutRef<"button"> & {
  label: string;
  visible: boolean;
  onHoverChange?: (hovered: boolean) => void;
}

const ProjectInlineAction = React.forwardRef<HTMLButtonElement, ProjectInlineActionProps>(function ProjectInlineAction({
  label,
  visible,
  onHoverChange,
  tabIndex,
  onClick,
  onMouseEnter,
  onMouseLeave,
  className,
  children,
  ...props
}, ref) {
  return (
    <Button
      {...props}
      ref={ref}
      type="button"
      variant="ghost"
      size="icon"
      aria-label={label}
      title={label}
      tabIndex={tabIndex ?? (visible ? undefined : -1)}
      className={cn(
        "absolute top-0 z-10 text-sidebar-foreground/45 opacity-0 transition-[color,opacity] duration-150 hover:bg-transparent hover:text-sidebar-foreground group-hover/project-row:opacity-100 data-[state=open]:text-sidebar-foreground dark:hover:bg-transparent",
        visible && "opacity-100",
        className,
      )}
      onMouseEnter={(event) => {
        onMouseEnter?.(event);
        onHoverChange?.(true);
      }}
      onMouseLeave={(event) => {
        onMouseLeave?.(event);
        onHoverChange?.(false);
      }}
      onClick={(event) => {
        onClick?.(event);
        event.preventDefault();
        event.stopPropagation();
      }}
    >
      {children}
    </Button>
  );
});

type ProjectSortableRenderProps = {
  attributes: ReturnType<typeof useSortable>["attributes"];
  listeners: ReturnType<typeof useSortable>["listeners"];
  isDragging: boolean;
}

function ProjectSortableItem({
  children,
  disabled,
  projectID,
}: {
  children: (props: ProjectSortableRenderProps) => React.ReactNode;
  disabled: boolean;
  projectID: string;
}) {
  const {
    attributes,
    isDragging,
    listeners,
    setNodeRef,
    transform,
    transition,
  } = useSortable({
    id: projectID,
    disabled,
  });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  } satisfies React.CSSProperties;

  return (
    <SidebarMenuItem
      ref={setNodeRef}
      data-sidebar-motion-key={`project-${projectID}`}
      style={style}
      className={cn("transition-opacity", isDragging && "opacity-45")}
    >
      {children({ attributes, isDragging, listeners })}
    </SidebarMenuItem>
  );
}

function ProjectDragHandle({
  attributes,
  disabled,
  hidden,
  label,
  listeners,
  visible,
}: {
  attributes: ProjectSortableRenderProps["attributes"];
  disabled: boolean;
  hidden: boolean;
  label: string;
  listeners: ProjectSortableRenderProps["listeners"];
  visible: boolean;
}) {
  const iconRef = React.useRef<GripVerticalIconHandle>(null);

  if (hidden) {
    return null;
  }

  return (
    <Button
      {...attributes}
      {...listeners}
      type="button"
      variant="ghost"
      size="icon"
      aria-label={label}
      title={label}
      disabled={disabled}
      className={cn(
        "absolute right-0 top-0 z-20 cursor-grab text-sidebar-foreground/45 opacity-0 transition-[color,opacity] duration-150 hover:bg-transparent hover:text-sidebar-foreground active:cursor-grabbing group-hover/project-row:opacity-100 disabled:cursor-not-allowed disabled:text-sidebar-foreground/40 dark:hover:bg-transparent",
        visible && "opacity-100",
      )}
      style={{ touchAction: "none" }}
      onMouseEnter={() => iconRef.current?.startAnimation()}
      onMouseLeave={() => iconRef.current?.stopAnimation()}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
      }}
    >
      <GripVerticalIcon aria-hidden ref={iconRef} size={14} className="size-4 text-current" />
    </Button>
  );
}

export function NavProjects() {
  const t = useTranslations("recent.projects");
  const tRecent = useTranslations("recent");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const { isMobile, setOpenMobile } = useSidebar();
  const router = useRouter();
  const onNavigate = useSidebarConversationNavigation();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const activeRecentProjectID = searchParams.get("project") ?? "";
  const activeChatProjectID = searchParams.get("project_id") ?? "";
  const activeProjectID = pathname === "/chat" ? activeChatProjectID : activeRecentProjectID;
  const activeConversationID = useLayoutActiveConversation();
  const { deleteFilesByDefault: deleteConversationFilesByDefault } = useSettingsChatPreferences();
  const { requestNewConversation } = useChatSession();
  const {
    items,
    projects,
    lastChange,
    createProject,
    updateProject,
    deleteProject,
    reorderProjects,
    renameByPublicID,
    regenerateTitleByPublicID,
    setStarByPublicID,
    setProjectByPublicID,
    archiveByPublicID,
    deleteByPublicID,
    touchByPublicID,
  } = useSidebarConversations();
  const [draft, setDraft] = React.useState<ProjectDraft | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<ProjectActionTarget | null>(null);
  const [deleteProjectConversations, setDeleteProjectConversations] = React.useState(false);
  const [deleteProjectFiles, setDeleteProjectFiles] = React.useState(false);
  const [conversationRenameTarget, setConversationRenameTarget] = React.useState<SidebarConversationRenameTarget>(null);
  const [conversationDeleteTarget, setConversationDeleteTarget] = React.useState<SidebarConversationDeleteTarget>(null);
  const [deleteConversationFiles, setDeleteConversationFiles] = React.useState(false);
  const [shareTarget, setShareTarget] = React.useState<{
    publicID: string;
    title: string;
  } | null>(null);
  const [renameValue, setRenameValue] = React.useState("");
  const [autoRenamingConversationID, setAutoRenamingConversationID] = React.useState<string | null>(null);
  const [openProjectMenuID, setOpenProjectMenuID] = React.useState<string | null>(null);
  const [hoveredProjectMenuID, setHoveredProjectMenuID] = React.useState<string | null>(null);
  const [hoveredProjectCreateID, setHoveredProjectCreateID] = React.useState<string | null>(null);
  const [hoveredProjectRowID, setHoveredProjectRowID] = React.useState<string | null>(null);
  const [focusedProjectRowID, setFocusedProjectRowID] = React.useState<string | null>(null);
  const [draggingProjectID, setDraggingProjectID] = React.useState<string | null>(null);
  const [savingProjectOrder, setSavingProjectOrder] = React.useState(false);
  const [projectsOpen, setProjectsOpen] = useStoredBoolean(PROJECTS_OPEN_STORAGE_KEY, true);
  const activeConversationProjectID = React.useMemo(
    () => items.find((item) => item.publicID === activeConversationID)?.projectID ?? "",
    [activeConversationID, items],
  );
  const {
    ensureProjectExpanded,
    expandedProjectIDs,
    loadProjectConversations,
    projectConversationState,
    removeProject,
    toggleProjectExpanded,
  } = useLayoutProjectConversations({
    activeProjectID,
    items,
    lastChange,
    projects,
  });
  const deleteProjectConversationsID = React.useId();
  const deleteProjectFilesID = React.useId();
  const deleteConversationFilesID = React.useId();
  const projectsContentID = React.useId();
  const stableDeleteTarget = useDialogSnapshot(deleteTarget);
  const stableConversationDeleteTarget = useDialogSnapshot(conversationDeleteTarget);
  const stableShareTarget = useDialogSnapshot(shareTarget);
  const onExportConversation = useConversationExport({
    successMessage: tRecent("exported"),
    failureMessage: tRecent("exportFailed"),
  });
  const projectIDs = React.useMemo(() => projects.map((project) => project.publicID), [projects]);
  const projectSortSensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 4,
      },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  const closeDraft = React.useCallback(() => {
    setDraft(null);
  }, []);

  const onRenameConversation = React.useCallback((publicID: string, currentTitle: string) => {
    setConversationRenameTarget({ publicID, currentTitle });
    setRenameValue(currentTitle);
  }, []);

  const onRenameConversationCancel = React.useCallback(() => {
    setConversationRenameTarget(null);
    setRenameValue("");
  }, []);

  const onRenameConversationCommit = React.useCallback(
    async (publicID: string, currentTitle: string) => {
      const nextTitle = renameValue.trim();
      if (!nextTitle || nextTitle === currentTitle) {
        onRenameConversationCancel();
        return;
      }
      await renameByPublicID(publicID, nextTitle);
      onRenameConversationCancel();
    },
    [onRenameConversationCancel, renameByPublicID, renameValue],
  );

  const onAutoRenameConversation = React.useCallback(
    async (publicID: string) => {
      if (autoRenamingConversationID) {
        return;
      }
      setAutoRenamingConversationID(publicID);
      try {
        const updated = await regenerateTitleByPublicID(publicID);
        if (updated) {
          onRenameConversationCancel();
        }
      } catch {
        // Keep the current rename input open so the user can retry or edit manually.
      } finally {
        setAutoRenamingConversationID(null);
      }
    },
    [autoRenamingConversationID, onRenameConversationCancel, regenerateTitleByPublicID],
  );

  const onArchiveConversation = React.useCallback(
    async (publicID: string) => {
      await archiveByPublicID(publicID, true);
      if (activeConversationID === publicID) {
        router.push("/chat");
      }
    },
    [activeConversationID, archiveByPublicID, router],
  );

  const onDeleteConversation = React.useCallback((publicID: string, title: string) => {
    setDeleteConversationFiles(deleteConversationFilesByDefault);
    setConversationDeleteTarget({ publicID, title });
  }, [deleteConversationFilesByDefault]);

  const confirmDeleteConversation = React.useCallback(async () => {
    if (!conversationDeleteTarget) {
      return;
    }
    const ok = await deleteByPublicID(conversationDeleteTarget.publicID, { deleteFiles: deleteConversationFiles });
    if (ok && activeConversationID === conversationDeleteTarget.publicID) {
      router.push("/chat");
    }
    setConversationDeleteTarget(null);
    setDeleteConversationFiles(false);
  }, [activeConversationID, conversationDeleteTarget, deleteByPublicID, deleteConversationFiles, router]);

  const startProjectConversation = React.useCallback(
    (projectID: string) => {
      ensureProjectExpanded(projectID, true);
      requestNewConversation({ projectID });
      router.push(`/chat?project_id=${encodeURIComponent(projectID)}`);
      if (isMobile) {
        setOpenMobile(false);
      }
    },
    [ensureProjectExpanded, isMobile, requestNewConversation, router, setOpenMobile],
  );

  const onProjectDragStart = React.useCallback((event: DragStartEvent) => {
    setDraggingProjectID(String(event.active.id));
  }, []);

  const onProjectDragEnd = React.useCallback(async (event: DragEndEvent) => {
    setDraggingProjectID(null);

    const { active, over } = event;
    if (!over || active.id === over.id || savingProjectOrder) {
      return;
    }

    const activeProjectID = String(active.id);
    const overProjectID = String(over.id);
    const fromIndex = projectIDs.indexOf(activeProjectID);
    const toIndex = projectIDs.indexOf(overProjectID);
    if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) {
      return;
    }

    const orderedProjectIDs = arrayMove(projectIDs, fromIndex, toIndex);
    setSavingProjectOrder(true);
    try {
      await reorderProjects(orderedProjectIDs);
    } catch (error) {
      toast.error(t("reorderFailed"), {
        description: resolveErrorMessage(error, t("reorderFailed")),
      });
    } finally {
      setSavingProjectOrder(false);
    }
  }, [projectIDs, reorderProjects, resolveErrorMessage, savingProjectOrder, t]);

  const onProjectDragCancel = React.useCallback(() => {
    setDraggingProjectID(null);
  }, []);

  const commitDraft = React.useCallback(async () => {
    const name = draft?.name.trim() ?? "";
    if (!draft || !name) {
      closeDraft();
      return;
    }
    if (draft.publicID) {
      await updateProject(draft.publicID, { name, systemPrompt: draft.systemPrompt.trim() });
    } else {
      await createProject({ name, systemPrompt: draft.systemPrompt.trim() });
    }
    closeDraft();
  }, [closeDraft, createProject, draft, updateProject]);

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget?.publicID) {
      return;
    }
    const deletingProjectID = deleteTarget.publicID;
    const deletingActiveConversation =
      deleteProjectConversations &&
      (
        projectConversationState[deletingProjectID]?.items.some((item) => item.publicID === activeConversationID) ||
        items.some((item) => item.projectID === deletingProjectID && item.publicID === activeConversationID)
      );
    const deleted = await deleteProject(deletingProjectID, {
      deleteConversations: deleteProjectConversations,
      deleteFiles: deleteProjectConversations && deleteProjectFiles,
    });
    if (deleted && pathname === "/recent" && activeProjectID === deleteTarget.publicID) {
      router.replace("/recent");
    }
    if (deleted && deletingActiveConversation) {
      router.push("/chat");
    }
    if (deleted) {
      removeProject(deletingProjectID);
    }
    setDeleteTarget(null);
    setDeleteProjectConversations(false);
    setDeleteProjectFiles(false);
  }, [
    activeConversationID,
    activeProjectID,
    deleteProject,
    deleteProjectConversations,
    deleteProjectFiles,
    deleteTarget,
    items,
    pathname,
    projectConversationState,
    removeProject,
    router,
  ]);

  React.useEffect(() => {
    if (!deleteTarget) {
      setDeleteProjectConversations(false);
      setDeleteProjectFiles(false);
    }
  }, [deleteTarget]);

  React.useEffect(() => {
    if (!deleteProjectConversations) {
      setDeleteProjectFiles(false);
    }
  }, [deleteProjectConversations]);

  React.useEffect(() => {
    if (!conversationDeleteTarget) {
      setDeleteConversationFiles(false);
    }
  }, [conversationDeleteTarget]);

  if (projects.length === 0) {
    return (
      <>
        <div className="relative z-10 group-data-[collapsible=icon]:pointer-events-none group-data-[collapsible=icon]:opacity-0">
          <Collapsible open={projectsOpen} onOpenChange={setProjectsOpen}>
            <SidebarGroup className="px-2 py-2">
              <ProjectGroupHeader
                title={t("title")}
                createLabel={t("create")}
                contentID={projectsContentID}
                open={projectsOpen}
                onCreate={() => setDraft({ name: "", systemPrompt: "" })}
                onOpenChange={setProjectsOpen}
                toggleLabel={projectsOpen ? t("collapseSection") : t("expandSection")}
              />
              <CollapsibleMotionContent id={projectsContentID} open={projectsOpen}>
                <div className="px-2 py-1 text-xs text-sidebar-foreground/55">{t("empty")}</div>
              </CollapsibleMotionContent>
            </SidebarGroup>
          </Collapsible>
        </div>
        <ProjectDialog draft={draft} setDraft={setDraft} onOpenChange={(open) => !open && closeDraft()} onSubmit={commitDraft} />
      </>
    );
  }

  return (
    <>
      <div className="relative z-10 group-data-[collapsible=icon]:pointer-events-none group-data-[collapsible=icon]:opacity-0">
        <Collapsible open={projectsOpen} onOpenChange={setProjectsOpen}>
          <SidebarGroup className="px-2 py-2">
            <ProjectGroupHeader
              title={t("title")}
              createLabel={t("create")}
              contentID={projectsContentID}
              open={projectsOpen}
              onCreate={() => setDraft({ name: "", systemPrompt: "" })}
              onOpenChange={setProjectsOpen}
              toggleLabel={projectsOpen ? t("collapseSection") : t("expandSection")}
            />
            <CollapsibleMotionContent id={projectsContentID} open={projectsOpen}>
              <DndContext
                sensors={projectSortSensors}
                collisionDetection={closestCenter}
                onDragStart={onProjectDragStart}
                onDragEnd={(event) => void onProjectDragEnd(event)}
                onDragCancel={onProjectDragCancel}
              >
                <SortableContext items={projectIDs} strategy={verticalListSortingStrategy}>
                  <SidebarMenu className="gap-0.5">
                    {projects.map((project) => {
                      const expanded = expandedProjectIDs.has(project.publicID);
                      const conversationState = projectConversationState[project.publicID];
                      const conversationLoading = expanded && (!conversationState || conversationState.loading);
                      const hasActiveChild = Boolean(conversationState?.items.some((item) => item.publicID === activeConversationID));
                      const active =
                        ((pathname === "/recent" || pathname === "/chat") && activeProjectID === project.publicID) ||
                        activeConversationProjectID === project.publicID ||
                        hasActiveChild;
                      const rowHovered = hoveredProjectRowID === project.publicID;
                      const rowFocused = focusedProjectRowID === project.publicID;
                      const createHovered = hoveredProjectCreateID === project.publicID;
                      const menuHovered = hoveredProjectMenuID === project.publicID;
                      const menuOpen = openProjectMenuID === project.publicID;
                      const rowDragging = draggingProjectID === project.publicID;
                      const canSortProjects = projects.length >= 2;
                      const projectActionPaddingClassName = canSortProjects ? "pr-24" : "pr-16";
                      const projectCreateActionClassName = canSortProjects ? "right-16" : "right-8";
                      const projectMenuActionClassName = canSortProjects ? "right-8" : "right-0";
                      const showProjectActions = isMobile || (!rowDragging && (rowHovered || rowFocused || menuHovered || menuOpen));
                      const projectConversationContentID = `sidebar-project-${project.publicID}-conversations`;
                      return (
                        <ProjectSortableItem
                          key={project.publicID}
                          projectID={project.publicID}
                          disabled={!canSortProjects || savingProjectOrder}
                        >
                          {({ attributes, isDragging, listeners }) => (
                            <>
                              <div
                                className="group/project-row relative"
                                onFocus={(event) => {
                                  setFocusedProjectRowID(
                                    event.target instanceof HTMLElement && event.target.matches(":focus-visible")
                                      ? project.publicID
                                      : null,
                                  );
                                }}
                                onBlur={(event) => {
                                  const nextTarget = event.relatedTarget;
                                  if (!(nextTarget instanceof Node) || !event.currentTarget.contains(nextTarget)) {
                                    setFocusedProjectRowID(null);
                                  }
                                }}
                              >
                                <ProjectDragHandle
                                  attributes={attributes}
                                  disabled={savingProjectOrder}
                                  hidden={!canSortProjects}
                                  label={t("dragToReorder", { name: project.name || t("untitled") })}
                                  listeners={listeners}
                                  visible={isMobile || rowHovered || rowFocused || isDragging}
                                />
                                <ProjectTreeButton
                                  active={active}
                                  actionPaddingClassName={projectActionPaddingClassName}
                                  contentID={projectConversationContentID}
                                  expanded={expanded}
                                  name={project.name}
                                  onHoverChange={(hovered) => setHoveredProjectRowID(hovered ? project.publicID : null)}
                                  onToggleExpanded={() => toggleProjectExpanded(project.publicID)}
                                />
                                <ProjectInlineAction
                                  label={t("newChatInProject")}
                                  visible={showProjectActions}
                                  className={projectCreateActionClassName}
                                  onHoverChange={(hovered) => setHoveredProjectCreateID(hovered ? project.publicID : null)}
                                  onClick={() => startProjectConversation(project.publicID)}
                                >
                                  <PlusIcon aria-hidden size={16} strokeWidth={1.6} animate={createHovered ? "default" : undefined} />
                                </ProjectInlineAction>
                                <DropdownMenu
                                  modal={false}
                                  open={menuOpen}
                                  onOpenChange={(open) => setOpenProjectMenuID(open ? project.publicID : null)}
                                >
                                  <DropdownMenuTrigger asChild>
                                    <ProjectInlineAction
                                      label={t("menu")}
                                      visible={showProjectActions}
                                      className={projectMenuActionClassName}
                                      onHoverChange={(hovered) => setHoveredProjectMenuID(hovered ? project.publicID : null)}
                                    >
                                      <Ellipsis aria-hidden size={16} strokeWidth={1.4} animate={menuHovered ? "pulse" : undefined} />
                                    </ProjectInlineAction>
                                  </DropdownMenuTrigger>
                                  <DropdownMenuContent align="end" className="w-max min-w-36 max-w-[calc(100vw-2rem)]">
                                    <DropdownMenuItem
                                      onSelect={(event) => {
                                        event.preventDefault();
                                        setDraft({ publicID: project.publicID, name: project.name, systemPrompt: project.systemPrompt ?? "" });
                                      }}
                                    >
                                      <DropdownMenuItemIcon icon={PencilLine} className="text-current" />
                                      {t("edit")}
                                    </DropdownMenuItem>
                                    <DropdownMenuSeparator />
                                    <DropdownMenuItem
                                      variant="destructive"
                                      onSelect={(event) => {
                                        event.preventDefault();
                                        setDeleteTarget({ publicID: project.publicID, name: project.name });
                                      }}
                                    >
                                      <DropdownMenuItemIcon icon={Trash} className="text-current" />
                                      {t("delete")}
                                    </DropdownMenuItem>
                                  </DropdownMenuContent>
                                </DropdownMenu>
                              </div>
                              <AnimatePresence initial={false}>
                                {expanded ? (
                                  <motion.div
                                    key={`${project.publicID}-conversations`}
                                    id={projectConversationContentID}
                                    initial={{ height: 0, opacity: 0, "--mask-stop": "0%", y: 6 }}
                                    animate={{ height: "auto", opacity: 1, "--mask-stop": "100%", y: 0 }}
                                    exit={{ height: 0, opacity: 0, "--mask-stop": "0%", y: 6 }}
                                    transition={PROJECT_TREE_ACCORDION_TRANSITION}
                                    style={PROJECT_TREE_ACCORDION_MASK_STYLE}
                                  >
                                    <SidebarMenuSub className="mx-0 w-full translate-x-0 gap-0.5 border-l-0 px-0 py-0.5">
                                      {conversationLoading ? (
                                        <SidebarMenuSubItem>
                                          <div className="flex h-7 w-full items-center gap-2 rounded-md pl-8 pr-2 text-xs text-muted-foreground">
                                            <Spinner className="size-3.5" />
                                            <span>{tRecent("loadingMore")}</span>
                                          </div>
                                        </SidebarMenuSubItem>
                                      ) : null}

                                      {conversationState?.error ? (
                                        <SidebarMenuSubItem>
                                          <Button
                                            type="button"
                                            variant="ghost"
                                            size="sm"
                                            className="w-full min-w-0 justify-start pl-8 pr-2 text-left text-xs font-normal text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                                            onClick={() => void loadProjectConversations(project.publicID, true)}
                                          >
                                            <span className="truncate">{tRecent("loadMoreFailed")}</span>
                                            <span className="ml-auto shrink-0 underline underline-offset-4">{tRecent("retry")}</span>
                                          </Button>
                                        </SidebarMenuSubItem>
                                      ) : null}

                                      {conversationState?.loaded && conversationState.items.length === 0 ? (
                                        <SidebarMenuSubItem>
                                          <div className="w-full rounded-md py-1 pl-8 pr-2 text-xs text-sidebar-foreground/55">
                                            {tRecent("empty")}
                                          </div>
                                        </SidebarMenuSubItem>
                                      ) : null}

                                      {conversationState?.items.map((conversation) => {
                                        const title = conversation.title || tRecent("untitled");
                                        return (
                                          <SidebarConversationItem
                                            key={conversation.publicID}
                                            active={activeConversationID === conversation.publicID}
                                            item={{
                                              publicID: conversation.publicID,
                                              title,
                                              url: `/chat?conversation_id=${conversation.publicID}`,
                                              shareActive:
                                                conversation.shareStatus === "active" && Boolean(conversation.shareID?.trim()),
                                            }}
                                            starAction={{
                                              label: conversation.isStarred ? tRecent("row.unstar") : tRecent("row.star"),
                                              icon: conversation.isStarred ? StarOff : Star,
                                              onSelect: (targetPublicID) => {
                                                void setStarByPublicID(targetPublicID, !conversation.isStarred);
                                              },
                                            }}
                                            projectMenu={{
                                              label: tRecent("row.moveToProject"),
                                              unassignedLabel: tRecent("projects.unassigned"),
                                              currentProjectID: conversation.projectID,
                                              projects,
                                              onSelect: (targetPublicID, targetProjectID) => {
                                                void setProjectByPublicID(targetPublicID, targetProjectID);
                                              },
                                            }}
                                            isTransferring={false}
                                            isRenaming={conversationRenameTarget?.publicID === conversation.publicID}
                                            renameValue={
                                              conversationRenameTarget?.publicID === conversation.publicID ? renameValue : title
                                            }
                                            rowClassName="w-full"
                                            linkClassName="pl-8"
                                            onRenameValueChange={setRenameValue}
                                            onRenameCommit={onRenameConversationCommit}
                                            onRenameCancel={onRenameConversationCancel}
                                            onAutoRename={onAutoRenameConversation}
                                            isAutoRenaming={autoRenamingConversationID === conversation.publicID}
                                            onRename={onRenameConversation}
                                            onArchive={onArchiveConversation}
                                            onShare={(publicID, shareTitle) => setShareTarget({ publicID, title: shareTitle })}
                                            onExport={onExportConversation}
                                            onDelete={onDeleteConversation}
                                            onNavigate={onNavigate}
                                            menuTriggerID={`project-conversation-menu-trigger-${conversation.publicID}`}
                                          />
                                        );
                                      })}
                                    </SidebarMenuSub>
                                  </motion.div>
                                ) : null}
                              </AnimatePresence>
                            </>
                          )}
                        </ProjectSortableItem>
                      );
                    })}
                  </SidebarMenu>
                </SortableContext>
              </DndContext>
            </CollapsibleMotionContent>
          </SidebarGroup>
        </Collapsible>
      </div>

      <ProjectDialog draft={draft} setDraft={setDraft} onOpenChange={(open) => !open && closeDraft()} onSubmit={commitDraft} />

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null);
            setDeleteProjectConversations(false);
            setDeleteProjectFiles(false);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDescription", { name: stableDeleteTarget?.name ?? t("untitled") })}
            </AlertDialogDescription>
            <div className="mt-1 flex items-start gap-2 py-2 text-left">
              <Checkbox
                id={deleteProjectConversationsID}
                checked={deleteProjectConversations}
                className="mt-0.5"
                onCheckedChange={(checked) => setDeleteProjectConversations(checked === true)}
              />
              <label htmlFor={deleteProjectConversationsID} className="cursor-pointer space-y-1">
                <span className="block text-xs font-medium text-foreground">{t("deleteConversationsLabel")}</span>
                <span className="block text-xs leading-5 text-muted-foreground">{t("deleteConversationsDescription")}</span>
              </label>
            </div>
            {deleteProjectConversations ? (
              <DeleteFilesOption
                id={deleteProjectFilesID}
                checked={deleteProjectFiles}
                onCheckedChange={setDeleteProjectFiles}
              />
            ) : null}
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void confirmDelete()}>
              {t("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={Boolean(conversationDeleteTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setConversationDeleteTarget(null);
            setDeleteConversationFiles(false);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{tRecent("dialogs.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {tRecent("dialogs.deleteDescription", {
                label: tRecent("deleteConversationLabel", { title: stableConversationDeleteTarget?.title || tRecent("untitled") }),
              })}
            </AlertDialogDescription>
            <DeleteFilesOption
              id={deleteConversationFilesID}
              checked={deleteConversationFiles}
              onCheckedChange={setDeleteConversationFiles}
            />
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tRecent("dialogs.cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void confirmDeleteConversation()}>
              {tRecent("dialogs.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {stableShareTarget ? (
        <ConversationShareDialog
          open={Boolean(shareTarget)}
          onOpenChange={(open) => !open && setShareTarget(null)}
          conversationPublicID={stableShareTarget.publicID}
          conversationTitle={stableShareTarget.title}
          onShareChange={(share) => {
            touchByPublicID(stableShareTarget.publicID, sharePatchFromDTO(share));
          }}
        />
      ) : null}
    </>
  );
}

function ProjectDialog({
  draft,
  setDraft,
  onOpenChange,
  onSubmit,
}: {
  draft: ProjectDraft | null;
  setDraft: (draft: ProjectDraft | null) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void | Promise<void>;
}) {
  const t = useTranslations("recent.projects");
  const [submitting, setSubmitting] = React.useState(false);
  const stableDraft = useDialogSnapshot(draft);

  React.useEffect(() => {
    if (!draft) {
      setSubmitting(false);
    }
  }, [draft]);

  const handleSubmit = React.useCallback<React.FormEventHandler<HTMLFormElement>>(
    async (event) => {
      event.preventDefault();
      if (!draft?.name.trim() || submitting) {
        return;
      }
      setSubmitting(true);
      try {
        await onSubmit();
      } finally {
        setSubmitting(false);
      }
    },
    [draft?.name, onSubmit, submitting],
  );

  return (
    <Dialog open={Boolean(draft)} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{stableDraft?.publicID ? t("editTitle") : t("createTitle")}</DialogTitle>
          <DialogDescription>{stableDraft?.publicID ? t("editDescription") : t("createDescription")}</DialogDescription>
        </DialogHeader>

        <motion.form layout transition={PROJECT_DIALOG_LAYOUT_TRANSITION} onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{t("nameLabel")}</p>
            <Input
              autoFocus
              value={stableDraft?.name ?? ""}
              maxLength={80}
              placeholder={t("namePlaceholder")}
              onChange={(event) => draft && setDraft({ ...draft, name: event.target.value })}
              disabled={submitting}
              required
            />
          </div>
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{t("systemPromptLabel")}</p>
            <Textarea
              value={stableDraft?.systemPrompt ?? ""}
              maxLength={12000}
              placeholder={t("systemPromptPlaceholder")}
              className="min-h-32 resize-y"
              onChange={(event) => draft && setDraft({ ...draft, systemPrompt: event.target.value })}
              disabled={submitting}
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={!draft?.name.trim() || submitting}>
              {t("save")}
            </Button>
          </DialogFooter>
        </motion.form>
      </DialogContent>
    </Dialog>
  );
}
