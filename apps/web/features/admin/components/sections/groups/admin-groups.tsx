"use client";

import * as React from "react";
import { ChevronRight, Plus, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogHeightTransition,
  DialogTitle,
} from "@/components/ui/dialog";
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableLoadingRow,
  TableRow,
} from "@/components/ui/table";
import { TablePagination, TableToolbar, type TableToolbarFilter } from "@/components/ui/table-tools";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { useAdminGroupsCreate } from "@/features/admin/hooks/use-admin-groups-create";
import { useAdminGroupsEditor } from "@/features/admin/hooks/use-admin-groups-editor";
import { useAdminGroups } from "@/features/admin/hooks/use-admin-groups";
import { resolveProtocolLabel, sortProtocolsForDisplay } from "@/features/admin/utils/llm-display";
import { ADAPTER_LABELS } from "@/features/admin/types/llm";
import type { AdminUserDTO } from "@/features/admin/api/admin-types";
import { cn } from "@/lib/utils";
import { parseProtocolsJSON } from "@/entities/model";
import { isString, parseJSON } from "@/shared/lib/type-guards";
import { GroupAccessPickerDialog } from "@/features/admin/components/sections/groups/groups-access-picker-dialog";
import { ModelAccessRulesPanel } from "@/features/admin/components/sections/groups/groups-model-access-rules-panel";
import { useAdminModelsPresentation } from "@/features/admin/hooks/use-admin-models-presentation";
import type { PermissionGroup } from "@/features/admin/api/permission-groups";

const GROUPS_PAGE_SIZE_DEFAULT = 25;

function parseStringArrayJSON(raw: string): string[] {
  if (!raw.trim()) {
    return [];
  }
  const parsed = parseJSON(raw);
  if (!Array.isArray(parsed)) {
    return [];
  }
  return parsed
    .filter(isString)
    .map((item) => item.trim())
    .filter(Boolean);
}

function useSubscriptionStatusLabel() {
  const t = useTranslations("adminUsers.subscriptionStatus");
  return React.useCallback(
    (value: string | null | undefined) => {
      switch (value?.trim()) {
        case "active":
          return t("active");
        case "trialing":
          return t("trialing");
        case "past_due":
          return t("pastDue");
        case "canceled":
          return t("canceled");
        case "unpaid":
          return t("unpaid");
        case "incomplete":
          return t("incomplete");
        case "incomplete_expired":
          return t("incompleteExpired");
        case "paused":
          return t("paused");
        case "free":
          return "";
        default:
          return value?.trim() || "";
      }
    },
    [t],
  );
}

function resolveUserSubscriptionLabel(
  user: AdminUserDTO,
  resolveSubscriptionStatusLabel: (value: string | null | undefined) => string,
): string {
  const planName = user.subscriptionPlanName.trim();
  const tier = user.subscriptionTier.trim();
  const status = resolveSubscriptionStatusLabel(user.subscriptionStatus);
  let planLabel = "";
  if (planName && planName !== "free") {
    planLabel = planName;
  } else if (tier && tier !== "free") {
    planLabel = tier;
  }

  if (planLabel && status) {
    return `${planLabel} · ${status}`;
  }
  return planLabel || status || "-";
}

export function AdminGroupsPage() {
  const t = useTranslations("adminGroups");
  const [query, setQueryState] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSizeState] = React.useState(GROUPS_PAGE_SIZE_DEFAULT);
  const { groups, loading, loadGroups, deletePending, deleteGroup } = useAdminGroups();
  const [createOpen, setCreateOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<PermissionGroup | null>(null);
  const [deleting, setDeleting] = React.useState<PermissionGroup | null>(null);
  const stableDeleting = useDialogSnapshot(deleting);

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);

  const setPageSize = React.useCallback((value: number) => {
    setPageSizeState(value);
    setPage(1);
  }, []);

  const filteredGroups = React.useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) {
      return groups;
    }
    return groups.filter((group) =>
      [group.name, group.description]
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(normalizedQuery)),
    );
  }, [groups, query]);

  const pageCount = Math.max(1, Math.ceil(filteredGroups.length / pageSize));

  React.useEffect(() => {
    setPage((current) => Math.min(Math.max(current, 1), pageCount));
  }, [pageCount]);

  const pagedGroups = React.useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredGroups.slice(start, start + pageSize);
  }, [filteredGroups, page, pageSize]);

  const handleDelete = React.useCallback(async (event?: React.MouseEvent<HTMLButtonElement>) => {
    event?.preventDefault();
    if (!deleting) {
      return;
    }
    const deletedGroupID = deleting.id;
    await deleteGroup(deleting, () => {
      setEditing((current) => (current?.id === deletedGroupID ? null : current));
      setDeleting(null);
    });
  }, [deleteGroup, deleting]);

  return (
    <div className="space-y-3 pb-10">
      <div className="flex h-10 items-center px-1">
        <h3 className="text-sm font-semibold">{t("title")}</h3>
      </div>

      <TableToolbar
        query={query}
        onQueryChange={setQuery}
        queryPlaceholder={t("searchPlaceholder")}
        loading={loading}
        onRefresh={() => void loadGroups()}
        refreshDisabled={loading}
        refreshLoading={loading}
      >
        <Button
          type="button"
          size="sm"
          className="h-7 gap-1 px-2 text-xs"
          onClick={() => setCreateOpen(true)}
          disabled={loading}
        >
          <Plus className="size-3.5 stroke-1" />
          {t("create")}
        </Button>
      </TableToolbar>

      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="min-w-[180px]">{t("name")}</TableHead>
            <TableHead className="min-w-[280px]">{t("descriptionField")}</TableHead>
            <TableHead className="w-[96px] text-center">{t("rateMultiplier")}</TableHead>
            <TableHead className="w-[140px] text-center">{t("modelCount")}</TableHead>
            <TableHead className="w-[160px] text-center">{t("coverageCount")}</TableHead>
            <TableHead className="w-[56px]" stickyEnd />
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading ? <TableLoadingRow colSpan={6} /> : null}
          {!loading && pagedGroups.length === 0 ? (
            <TableEmptyRow colSpan={6}>{t("noGroups")}</TableEmptyRow>
          ) : null}
          {!loading
            ? pagedGroups.map((group) => (
                <TableRow
                  key={group.id}
                  className="cursor-pointer"
                  onClick={() => setEditing(group)}
                >
                  <TableCell className="py-1.5 whitespace-nowrap">
                    <div className="flex h-7 items-center gap-2">
                      <span className="font-medium">{group.name}</span>
                      {group.isDefault ? (
                        <Badge variant="secondary">
                          {t("default")}
                        </Badge>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell className="max-w-[360px] py-1.5 text-muted-foreground">
                    <div className="truncate" title={group.description}>
                      {group.description || "-"}
                    </div>
                  </TableCell>
                  <TableCell className="py-1.5 text-center whitespace-nowrap tabular-nums">
                    <span className="flex h-7 items-center justify-center">
                      {(group.rateMultiplierPercent || 100) / 100}
                    </span>
                  </TableCell>
                  <TableCell className="py-1.5 text-center whitespace-nowrap tabular-nums">
                    <div className="flex min-h-7 flex-col items-center justify-center leading-4">
                      <span>{group.modelCount ?? 0}</span>
                      <span className="text-[11px] text-muted-foreground">
                        {t("groupModelBreakdown", {
                          manual: group.manualModelCount ?? 0,
                          automatic: group.ruleModelCount ?? 0,
                        })}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className="py-1.5 text-center whitespace-nowrap tabular-nums">
                    <div className="flex min-h-7 flex-col items-center justify-center leading-4">
                      <span>{group.userCount ?? 0}</span>
                      <span className="text-[11px] text-muted-foreground">
                        {group.isDefault
                          ? t("defaultCoverage")
                          : t("groupCoverageBreakdown", {
                              manual: group.manualUserCount ?? 0,
                              subscription: group.subscriptionUserCount ?? 0,
                            })}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className="w-[56px] py-1.5 whitespace-nowrap" stickyEnd>
                    <div className="flex h-7 items-center justify-end">
                      <Button
                        type="button"
                        size="icon-sm"
                        variant="ghost"
                        className="text-muted-foreground shadow-none"
                        disabled={group.isDefault}
                        aria-label={group.isDefault ? t("cannotDeleteDefault") : t("deleteGroup")}
                        title={group.isDefault ? t("cannotDeleteDefault") : t("deleteGroup")}
                        onClick={(event) => {
                          event.stopPropagation();
                          setDeleting(group);
                        }}
                      >
                        <Trash2 className="size-3.5 stroke-1" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            : null}
        </TableBody>
      </Table>

      <TablePagination
        total={filteredGroups.length}
        page={page}
        pageCount={pageCount}
        pageSize={pageSize}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
        loading={loading}
      />

      <CreateGroupDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={loadGroups}
      />

      <GroupEditSheet
        group={editing}
        onOpenChange={(open) => {
          if (!open) {
            setEditing(null);
          }
        }}
        onSaved={loadGroups}
      />

      <AlertDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open && !deletePending) {
            setDeleting(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("confirmDeleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {stableDeleting
                ? t("confirmDeleteWithImpact", {
                    models: stableDeleting.manualModelCount ?? 0,
                    rules: stableDeleting.ruleModelCount ?? 0,
                    users: stableDeleting.manualUserCount ?? 0,
                  })
                : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletePending}>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction disabled={deletePending} onClick={handleDelete}>
              {deletePending ? <SpinnerLabel>{t("deleteGroup")}</SpinnerLabel> : t("deleteGroup")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function CreateGroupDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => Promise<void>;
}) {
  const t = useTranslations("adminGroups");
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [rateMultiplier, setRateMultiplier] = React.useState("1");
  const { saving, createGroup } = useAdminGroupsCreate({ onOpenChange, onCreated });

  React.useEffect(() => {
    if (open) {
      setName("");
      setDescription("");
      setRateMultiplier("1");
    }
  }, [open]);

  const handleCreate = React.useCallback<React.FormEventHandler<HTMLFormElement>>(async (event) => {
    event.preventDefault();
    await createGroup({ name, description, rateMultiplier });
  }, [createGroup, description, name, rateMultiplier]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0 sm:max-w-[560px]">
        <DialogHeightTransition contentClassName="max-h-[min(86vh,760px)]">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{t("createGroup")}</DialogTitle>
            <DialogDescription>{t("createGroupDescription")}</DialogDescription>
          </DialogHeader>

          <form onSubmit={handleCreate} className="flex min-h-0 flex-1 flex-col">
            <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground" htmlFor="group-name">
                  {t("name")}
                </Label>
                <Input
                  id="group-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  disabled={saving}
                  required
                />
              </div>

              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground" htmlFor="group-desc">
                  {t("descriptionField")}
                </Label>
                <Textarea
                  id="group-desc"
                  value={description}
                  onChange={(event) => setDescription(event.target.value)}
                  disabled={saving}
                />
              </div>

              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground" htmlFor="group-rate">
                  {t("rateMultiplier")}
                </Label>
                <Input
                  id="group-rate"
                  type="number"
                  min="0"
                  step="0.01"
                  value={rateMultiplier}
                  onChange={(event) => setRateMultiplier(event.target.value)}
                  disabled={saving}
                />
                <p className="text-xs text-muted-foreground">{t("rateMultiplierHint")}</p>
              </div>
            </div>

            <DialogFooter className="shrink-0 px-4 py-3">
              <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={saving}>
                {t("cancel")}
              </Button>
              <Button type="submit" disabled={saving || !name.trim()}>
                {saving ? <SpinnerLabel>{t("createGroup")}</SpinnerLabel> : t("createGroup")}
              </Button>
            </DialogFooter>
          </form>
        </DialogHeightTransition>
      </DialogContent>
    </Dialog>
  );
}

function GroupEditSheet({
  group,
  onOpenChange,
  onSaved,
}: {
  group: PermissionGroup | null;
  onOpenChange: (open: boolean) => void;
  onSaved: () => Promise<void>;
}) {
  const t = useTranslations("adminGroups");
  const resolveSubscriptionStatusLabel = useSubscriptionStatusLabel();
  const modelPresentation = useAdminModelsPresentation();
  const {
    name,
    setName,
    description,
    setDescription,
    rateMultiplier,
    setRateMultiplier,
    modelIDs,
    setModelIDs,
    modelRules,
    setModelRules,
    userIDs,
    setUserIDs,
    selectionLoading,
    selectionLoaded,
    modelRows,
    modelTotal,
    modelPage,
    setModelPage,
    modelPageSize,
    modelQuery,
    modelUpstreamOptions,
    modelUpstreamFilter,
    modelVendorFilter,
    modelProtocolFilter,
    modelLoading,
    modelBulkLoading,
    userRows,
    userTotal,
    userPage,
    setUserPage,
    userPageSize,
    userQuery,
    userSubscriptionFilter,
    userIdentityFilter,
    userIdentityProviderOptions,
    userLoading,
    userBulkLoading,
    saving,
    handleModelQueryChange,
    handleModelUpstreamFilterChange,
    handleModelVendorFilterChange,
    handleModelProtocolFilterChange,
    handleUserQueryChange,
    handleUserSubscriptionFilterChange,
    handleUserIdentityFilterChange,
    handleModelPageSizeChange,
    handleUserPageSizeChange,
    refreshModels,
    refreshUsers,
    selectAllModels,
    selectAllUsers,
    clearModelSelection,
    clearUserSelection,
    handleSave,
  } = useAdminGroupsEditor({ group, onOpenChange, onSaved });
  const [accessDialog, setAccessDialog] = React.useState<"models" | "users" | null>(null);
  const stableGroup = useDialogSnapshot(group);

  React.useEffect(() => {
    if (!group) {
      setAccessDialog(null);
    }
  }, [group]);

  const modelItems = React.useMemo(
    () =>
      modelRows.map((model) => {
        const protocols = sortProtocolsForDisplay(parseProtocolsJSON(model.protocolsJSON))
          .map((protocol) => resolveProtocolLabel(protocol));
        const upstreamNames = parseStringArrayJSON(model.upstreamNamesJSON);
        return {
          id: model.id,
          label: model.platformModelName,
          sourceLabels: upstreamNames,
          vendorLabels: model.vendor ? [model.vendor] : [],
          protocolLabels: protocols,
        };
      }),
    [modelRows],
  );

  const modelVendorOptions = React.useMemo(
    () => modelPresentation.vendors.map((vendor) => ({ value: vendor.key, label: vendor.name })),
    [modelPresentation.vendors],
  );

  const userItems = React.useMemo(
    () =>
      userRows.map((user) => ({
        id: user.id,
        label: user.username || user.publicID,
        nickname: user.displayName || "-",
        email: user.email || "-",
        subscriptionStatus: resolveUserSubscriptionLabel(user, resolveSubscriptionStatusLabel),
        identityProviders: user.identityProviders ?? [],
      })),
    [resolveSubscriptionStatusLabel, userRows],
  );

  const modelFilters = React.useMemo<TableToolbarFilter[]>(
    () => [
      {
        key: "upstream",
        label: t("upstreams"),
        value: modelUpstreamFilter,
        onValueChange: handleModelUpstreamFilterChange,
        options: [
          { label: t("allUpstreams"), value: "" },
          ...modelUpstreamOptions.map((upstream) => ({
            label: upstream.name,
            value: String(upstream.id),
          })),
        ],
      },
      {
        key: "vendor",
        label: t("modelVendor"),
        value: modelVendorFilter,
        onValueChange: handleModelVendorFilterChange,
        options: [
          { label: t("allVendors"), value: "" },
          ...modelVendorOptions,
        ],
      },
      {
        key: "protocol",
        label: t("protocols"),
        value: modelProtocolFilter,
        onValueChange: handleModelProtocolFilterChange,
        options: [
          { label: t("allProtocols"), value: "" },
          ...Object.entries(ADAPTER_LABELS).map(([value, label]) => ({ label, value })),
        ],
      },
    ],
    [
      handleModelProtocolFilterChange,
      handleModelUpstreamFilterChange,
      handleModelVendorFilterChange,
      modelProtocolFilter,
      modelUpstreamFilter,
      modelUpstreamOptions,
      modelVendorFilter,
      modelVendorOptions,
      t,
    ],
  );

  const userFilters = React.useMemo<TableToolbarFilter[]>(
    () => [
      {
        key: "subscriptionStatus",
        label: t("subscriptionStatus"),
        value: userSubscriptionFilter,
        onValueChange: handleUserSubscriptionFilterChange,
        options: [
          { label: t("allSubscriptions"), value: "" },
          { label: t("activeSubscription"), value: "active" },
          { label: t("freeSubscription"), value: "free" },
        ],
      },
      {
        key: "identityProvider",
        label: t("identitySource"),
        value: userIdentityFilter,
        onValueChange: handleUserIdentityFilterChange,
        options: [
          { label: t("allIdentitySources"), value: "" },
          ...userIdentityProviderOptions.map((provider) => ({
            label: provider.name || provider.slug,
            value: provider.slug,
          })),
        ],
      },
    ],
    [
      handleUserIdentityFilterChange,
      handleUserSubscriptionFilterChange,
      t,
      userIdentityFilter,
      userIdentityProviderOptions,
      userSubscriptionFilter,
    ],
  );

  const isDefaultGroup = stableGroup?.isDefault ?? false;
  const groupUserCount = stableGroup?.userCount ?? 0;
  const subscriptionUserCount = stableGroup?.subscriptionUserCount ?? 0;

  return (
    <>
      <Sheet open={group !== null} onOpenChange={onOpenChange}>
        <SheetContent className="gap-0">
          <SheetHeader className="shrink-0 px-4 py-4">
            <SheetTitle>{t("editGroup")}</SheetTitle>
            <SheetDescription className="sr-only">{t("editGroupDescription")}</SheetDescription>
          </SheetHeader>

          <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-4 pb-4">
            <GroupSheetSection title={t("basicInfo")} divided={false}>
              <div className="space-y-3">
                <div className="space-y-1">
                  <Label className="text-xs font-normal text-muted-foreground" htmlFor="edit-name">
                    {t("name")}
                  </Label>
                  <Input id="edit-name" value={name} onChange={(event) => setName(event.target.value)} />
                </div>
                <div className="space-y-1">
                  <Label className="text-xs font-normal text-muted-foreground" htmlFor="edit-desc">
                    {t("descriptionField")}
                  </Label>
                  <Textarea
                    id="edit-desc"
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                    className="min-h-20 resize-none"
                  />
                </div>
                <div className="space-y-1">
                  <Label className="text-xs font-normal text-muted-foreground" htmlFor="edit-rate">
                    {t("rateMultiplier")}
                  </Label>
                  <Input
                    id="edit-rate"
                    type="number"
                    min="0"
                    step="0.01"
                    value={rateMultiplier}
                    onChange={(event) => setRateMultiplier(event.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">{t("rateMultiplierHint")}</p>
                </div>
              </div>
            </GroupSheetSection>

            <GroupSheetSection title={t("accessScope")}>
              <div className="grid gap-2">
                <AccessScopeItem
                  title={t("modelAccess")}
                  count={modelIDs.size}
                  countLabel={t("modelAccessEditingSummary", {
                    manual: modelIDs.size,
                    rules: modelRules.length,
                  })}
                  loading={selectionLoading}
                  onConfigure={() => setAccessDialog("models")}
                />
                <AccessScopeItem
                  title={t("userAccess")}
                  count={groupUserCount}
                  countLabel={
                    isDefaultGroup
                      ? t("defaultGroupAllUsers", { count: groupUserCount })
                      : t("groupUserAccessEditingSummary", {
                          manual: userIDs.size,
                          subscription: subscriptionUserCount,
                        })
                  }
                  loading={selectionLoading}
                  onConfigure={isDefaultGroup ? undefined : () => setAccessDialog("users")}
                />
              </div>
            </GroupSheetSection>
          </div>

          <SheetFooter className="flex flex-row justify-end gap-2 px-4 py-3">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={saving}>
              {t("cancel")}
            </Button>
            <Button
              type="button"
              onClick={handleSave}
              disabled={saving || selectionLoading || !selectionLoaded || !name.trim()}
            >
              {saving ? <SpinnerLabel>{t("save")}</SpinnerLabel> : t("save")}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <GroupAccessPickerDialog
        open={accessDialog === "models"}
        onOpenChange={(open) => !open && setAccessDialog(null)}
        title={t("configureModels")}
        description={t("configureModelsDescription")}
        items={modelItems}
        selectedIDs={modelIDs}
        setSelectedIDs={setModelIDs}
        query={modelQuery}
        onQueryChange={handleModelQueryChange}
        filters={modelFilters}
        topContent={
          <ModelAccessRulesPanel
            rules={modelRules}
            onRulesChange={setModelRules}
            upstreamOptions={modelUpstreamOptions}
            vendorOptions={modelVendorOptions}
            disabled={selectionLoading || modelLoading || modelBulkLoading}
          />
        }
        manualTitle={t("manualRules")}
        page={modelPage}
        pageSize={modelPageSize}
        total={modelTotal}
        loading={modelLoading || selectionLoading}
        bulkLoading={modelBulkLoading}
        onPageChange={setModelPage}
        onPageSizeChange={handleModelPageSizeChange}
        onRefresh={refreshModels}
        onSelectAllResults={selectAllModels}
        onClearSelection={clearModelSelection}
        searchPlaceholder={t("searchModels")}
        itemTitle={t("models")}
        sourceTitle={t("upstreams")}
        vendorTitle={t("modelVendor")}
        protocolTitle={t("protocols")}
        contentClassName="sm:max-w-[860px]"
        tableViewportClassName="max-h-[240px]"
        emptyText={t("noModels")}
      />

      <GroupAccessPickerDialog
        open={accessDialog === "users"}
        onOpenChange={(open) => !open && setAccessDialog(null)}
        title={t("configureUsers")}
        description={t("configureUsersDescription")}
        items={userItems}
        selectedIDs={userIDs}
        setSelectedIDs={setUserIDs}
        query={userQuery}
        onQueryChange={handleUserQueryChange}
        filters={userFilters}
        page={userPage}
        pageSize={userPageSize}
        total={userTotal}
        loading={userLoading || selectionLoading}
        bulkLoading={userBulkLoading}
        onPageChange={setUserPage}
        onPageSizeChange={handleUserPageSizeChange}
        onRefresh={refreshUsers}
        onSelectAllResults={selectAllUsers}
        onClearSelection={clearUserSelection}
        searchPlaceholder={t("searchUsers")}
        itemTitle={t("username")}
        nicknameTitle={t("nickname")}
        emailTitle={t("email")}
        subscriptionTitle={t("subscriptionStatus")}
        identityTitle={t("identitySource")}
        contentClassName="sm:max-w-[820px]"
        tableViewportClassName="max-h-[420px]"
        emptyText={t("noUsers")}
      />
    </>
  );
}

function GroupSheetSection({
  title,
  children,
  divided = true,
}: {
  title: string;
  children: React.ReactNode;
  divided?: boolean;
}) {
  return (
    <section className={cn("space-y-3", divided && "border-t pt-5")}>
      <h3 className="text-xs font-medium text-foreground">{title}</h3>
      {children}
    </section>
  );
}

function AccessScopeItem({
  title,
  count,
  countLabel,
  loading,
  onConfigure,
}: {
  title: string;
  count: number;
  countLabel?: string;
  loading: boolean;
  onConfigure?: () => void;
}) {
  const t = useTranslations("adminGroups");

  return (
    <div className="flex min-h-12 items-center justify-between gap-3 rounded-md bg-muted/30 px-3 py-2.5">
      <div className="min-w-0 flex-1">
        <p className="truncate text-xs text-foreground">{title}</p>
        <p className="truncate text-[11px] text-muted-foreground">
          {loading ? t("loading") : (countLabel ?? t("accessSelected", { count }))}
        </p>
      </div>
      {onConfigure ? (
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-7 shrink-0 gap-1 px-2 text-xs text-muted-foreground shadow-none hover:bg-background/80 hover:text-foreground"
          disabled={loading}
          onClick={onConfigure}
          aria-label={`${t("configure")} ${title}`}
        >
          {t("configure")}
          <ChevronRight className="size-3.5 stroke-1" />
        </Button>
      ) : null}
    </div>
  );
}
