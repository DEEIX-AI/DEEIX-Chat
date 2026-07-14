"use client";

import * as React from "react";
import { ChevronRight, Plus, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

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
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { invalidateAdminReferenceDataCache } from "@/features/admin/api/reference-data";
import { listAllAdminPages } from "@/features/admin/api/shared";
import { listAdminIdentityProviders } from "@/features/admin/api/auth";
import { listAdminLLMModels, listAdminLLMUpstreams } from "@/features/admin/api/llm";
import { listAdminUsers } from "@/features/admin/api/accounts";
import { resolveProtocolLabel, sortProtocolsForDisplay } from "@/features/admin/utils/llm-display";
import { ADAPTER_LABELS } from "@/features/admin/types/llm";
import type { AdminLLMModelDTO, AdminLLMUpstreamView } from "@/features/admin/api/llm.types";
import type { IdentityProviderDTO, UserDTO } from "@/shared/api/auth.types";
import { cn } from "@/lib/utils";
import { parseProtocolsJSON } from "@/shared/lib/model-protocols";
import { KNOWN_VENDOR_OPTIONS } from "@/shared/lib/model-identity";
import { GroupAccessPickerDialog } from "@/features/admin/components/sections/groups/group-access-picker-dialog";
import { ModelAccessRulesPanel } from "@/features/admin/components/sections/groups/model-access-rules-panel";
import {
  createPermissionGroup,
  deletePermissionGroup,
  listGroupModels,
  listGroupUsers,
  listPermissionGroups,
  setGroupModels,
  setGroupUsers,
  updatePermissionGroup,
  type PermissionGroup,
  type PermissionGroupModelRule,
} from "@/features/admin/api/permission-groups";

const GROUP_PICKER_PAGE_SIZE_DEFAULT = 25;
const GROUPS_PAGE_SIZE_DEFAULT = 25;

function parseStringArrayJSON(raw: string): string[] {
  if (!raw.trim()) {
    return [];
  }
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed
      .filter((item): item is string => typeof item === "string")
      .map((item) => item.trim())
      .filter(Boolean);
  } catch {
    return [];
  }
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
  user: UserDTO,
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
  const [groups, setGroups] = React.useState<PermissionGroup[]>([]);
  const [query, setQueryState] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSizeState] = React.useState(GROUPS_PAGE_SIZE_DEFAULT);
  const [loading, setLoading] = React.useState(true);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<PermissionGroup | null>(null);
  const [deleting, setDeleting] = React.useState<PermissionGroup | null>(null);
  const stableDeleting = useDialogSnapshot(deleting);
  const [deletePending, setDeletePending] = React.useState(false);

  const fetchGroups = React.useCallback(async () => {
    const token = await resolveAccessToken();
    return listPermissionGroups(token);
  }, []);

  const loadGroups = React.useCallback(async (options: { showLoading?: boolean } = {}) => {
    const showLoading = options.showLoading ?? true;
    if (showLoading) {
      setLoading(true);
    }
    try {
      setGroups(await fetchGroups());
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
    } finally {
      if (showLoading) {
        setLoading(false);
      }
    }
  }, [fetchGroups, t]);

  React.useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await fetchGroups();
        if (cancelled) {
          return;
        }
        setGroups(list);
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchGroups, t]);

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
    if (!deleting || deletePending) {
      return;
    }
    const deletedGroupID = deleting.id;
    setDeletePending(true);
    try {
      const token = await resolveAccessToken();
      const result = await deletePermissionGroup(token, deletedGroupID);
      toast.success(t("deletedWithSummary", {
        models: result.summary.manualModelCount ?? 0,
        rules: result.summary.ruleCount ?? 0,
        users: result.summary.manualUserCount ?? 0,
      }));
      invalidateAdminReferenceDataCache();
      setGroups((current) => current.filter((group) => group.id !== deletedGroupID));
      setEditing((current) => (current?.id === deletedGroupID ? null : current));
      setDeleting(null);
      void loadGroups({ showLoading: false });
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    } finally {
      setDeletePending(false);
    }
  }, [deletePending, deleting, loadGroups, t]);

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
            <TableHead>{t("name")}</TableHead>
            <TableHead>{t("descriptionField")}</TableHead>
            <TableHead className="text-right">{t("rateMultiplier")}</TableHead>
            <TableHead className="text-right">{t("modelCount")}</TableHead>
            <TableHead className="text-right">{t("coverageCount")}</TableHead>
            <TableHead className="w-16" />
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
                    {group.name}
                    {group.isDefault ? (
                      <Badge variant="secondary" className="ml-2">
                        {t("default")}
                      </Badge>
                    ) : null}
                  </TableCell>
                  <TableCell className="max-w-xs truncate py-1.5 text-muted-foreground">
                    {group.description}
                  </TableCell>
                  <TableCell className="py-1.5 text-right whitespace-nowrap">
                    {(group.rateMultiplierPercent || 100) / 100}
                  </TableCell>
                  <TableCell className="py-1.5 text-right whitespace-nowrap">
                    <div className="space-y-0.5">
                      <div>{group.modelCount ?? 0}</div>
                      <div className="text-[11px] text-muted-foreground">
                        {t("groupModelBreakdown", {
                          manual: group.manualModelCount ?? 0,
                          automatic: group.ruleModelCount ?? 0,
                        })}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="py-1.5 text-right whitespace-nowrap">
                    <div className="space-y-0.5">
                      <div>{group.userCount ?? 0}</div>
                      <div className="text-[11px] text-muted-foreground">
                        {group.isDefault
                          ? t("defaultCoverage")
                          : t("groupCoverageBreakdown", {
                              manual: group.manualUserCount ?? 0,
                              subscription: group.subscriptionUserCount ?? 0,
                            })}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="py-1.5 text-right whitespace-nowrap">
                    <Button
                      size="icon"
                      variant="ghost"
                      disabled={group.isDefault}
                      title={group.isDefault ? t("cannotDeleteDefault") : t("deleteGroup")}
                      onClick={(event) => {
                        event.stopPropagation();
                        setDeleting(group);
                      }}
                    >
                      <Trash2 className="size-4" />
                    </Button>
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
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (open) {
      setName("");
      setDescription("");
      setRateMultiplier("1");
    }
  }, [open]);

  const handleCreate = React.useCallback<React.FormEventHandler<HTMLFormElement>>(async (event) => {
    event.preventDefault();
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      const parsed = parseFloat(rateMultiplier);
      const rateMultiplierPercent =
        Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 100) : 100;
      await createPermissionGroup(token, { name, description, rateMultiplierPercent });
      toast.success(t("created"));
      onOpenChange(false);
      await onCreated();
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    } finally {
      setSaving(false);
    }
  }, [description, name, rateMultiplier, onCreated, onOpenChange, t]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
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
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [rateMultiplier, setRateMultiplier] = React.useState("1");
  const [modelIDs, setModelIDs] = React.useState<Set<number>>(new Set());
  const [modelRules, setModelRules] = React.useState<PermissionGroupModelRule[]>([]);
  const [userIDs, setUserIDs] = React.useState<Set<number>>(new Set());
  const [selectionLoading, setSelectionLoading] = React.useState(false);
  const [selectionLoaded, setSelectionLoaded] = React.useState(false);
  const [modelRows, setModelRows] = React.useState<AdminLLMModelDTO[]>([]);
  const [modelTotal, setModelTotal] = React.useState(0);
  const [modelPage, setModelPage] = React.useState(1);
  const [modelPageSize, setModelPageSizeState] = React.useState(GROUP_PICKER_PAGE_SIZE_DEFAULT);
  const [modelQuery, setModelQuery] = React.useState("");
  const [modelUpstreamOptions, setModelUpstreamOptions] = React.useState<AdminLLMUpstreamView[]>([]);
  const [modelUpstreamFilter, setModelUpstreamFilter] = React.useState("");
  const [modelVendorFilter, setModelVendorFilter] = React.useState("");
  const [modelProtocolFilter, setModelProtocolFilter] = React.useState("");
  const [modelReloadKey, setModelReloadKey] = React.useState(0);
  const [modelLoading, setModelLoading] = React.useState(false);
  const [modelBulkLoading, setModelBulkLoading] = React.useState(false);
  const [userRows, setUserRows] = React.useState<UserDTO[]>([]);
  const [userTotal, setUserTotal] = React.useState(0);
  const [userPage, setUserPage] = React.useState(1);
  const [userPageSize, setUserPageSizeState] = React.useState(GROUP_PICKER_PAGE_SIZE_DEFAULT);
  const [userQuery, setUserQuery] = React.useState("");
  const [userSubscriptionFilter, setUserSubscriptionFilter] = React.useState("");
  const [userIdentityFilter, setUserIdentityFilter] = React.useState("");
  const [userIdentityProviderOptions, setUserIdentityProviderOptions] = React.useState<IdentityProviderDTO[]>([]);
  const [userReloadKey, setUserReloadKey] = React.useState(0);
  const [userLoading, setUserLoading] = React.useState(false);
  const [userBulkLoading, setUserBulkLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [accessDialog, setAccessDialog] = React.useState<"models" | "users" | null>(null);
  const stableGroup = useDialogSnapshot(group);

  React.useEffect(() => {
    if (!group) {
      setSelectionLoading(false);
      setModelLoading(false);
      setModelBulkLoading(false);
      setUserLoading(false);
      setUserBulkLoading(false);
      setAccessDialog(null);
      return;
    }
    setName(group.name);
    setDescription(group.description);
    setRateMultiplier(String((group.rateMultiplierPercent || 100) / 100));
    setSelectionLoading(true);
    setSelectionLoaded(false);
    setModelRules([]);
    setModelRows([]);
    setModelTotal(0);
    setModelPage(1);
    setModelPageSizeState(GROUP_PICKER_PAGE_SIZE_DEFAULT);
    setModelQuery("");
    setModelUpstreamOptions([]);
    setModelUpstreamFilter("");
    setModelVendorFilter("");
    setModelProtocolFilter("");
    setModelReloadKey(0);
    setUserRows([]);
    setUserTotal(0);
    setUserPage(1);
    setUserPageSizeState(GROUP_PICKER_PAGE_SIZE_DEFAULT);
    setUserQuery("");
    setUserSubscriptionFilter("");
    setUserIdentityFilter("");
    setUserIdentityProviderOptions([]);
    setUserReloadKey(0);
    let cancelled = false;
    (async () => {
      try {
        const token = await resolveAccessToken();
        const [selectedModels, selectedUsers, upstreams, identityProviderPage] = await Promise.all([
          listGroupModels(token, group.id),
          group.isDefault ? Promise.resolve([]) : listGroupUsers(token, group.id),
          listAllAdminPages((options) =>
            listAdminLLMUpstreams(token, {
              ...options,
              status: "active",
              sort: "name_asc",
            }),
          ),
          listAdminIdentityProviders(token),
        ]);
        if (!cancelled) {
          setModelIDs(new Set(selectedModels.modelIDs));
          setModelRules(selectedModels.rules);
          setUserIDs(new Set(selectedUsers));
          setModelUpstreamOptions(upstreams);
          setUserIdentityProviderOptions(identityProviderPage.results);
          setSelectionLoaded(true);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      } finally {
        if (!cancelled) {
          setSelectionLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [group, t]);

  React.useEffect(() => {
    if (!group) {
      return;
    }
    let cancelled = false;
    setModelLoading(true);
    (async () => {
      try {
        const token = await resolveAccessToken();
        const page = await listAdminLLMModels(token, {
          page: modelPage,
          pageSize: modelPageSize,
          query: modelQuery.trim(),
          upstream: modelUpstreamFilter,
          vendor: modelVendorFilter,
          protocol: modelProtocolFilter,
        });
        if (!cancelled) {
          setModelRows(page.results);
          setModelTotal(page.total);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      } finally {
        if (!cancelled) {
          setModelLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [
    group,
    modelPage,
    modelPageSize,
    modelQuery,
    modelReloadKey,
    modelUpstreamFilter,
    modelVendorFilter,
    modelProtocolFilter,
    t,
  ]);

  React.useEffect(() => {
    if (!group) {
      return;
    }
    if (group.isDefault) {
      setUserRows([]);
      setUserTotal(group.userCount ?? 0);
      setUserLoading(false);
      return;
    }
    let cancelled = false;
    setUserLoading(true);
    (async () => {
      try {
        const token = await resolveAccessToken();
        const page = await listAdminUsers(token, {
          page: userPage,
          pageSize: userPageSize,
          query: userQuery.trim(),
          subscriptionStatus: userSubscriptionFilter,
          identityProvider: userIdentityFilter,
        });
        if (!cancelled) {
          setUserRows(page.results);
          setUserTotal(page.total);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      } finally {
        if (!cancelled) {
          setUserLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [group, t, userIdentityFilter, userPage, userPageSize, userQuery, userReloadKey, userSubscriptionFilter]);

  const handleModelQueryChange = React.useCallback((value: string) => {
    setModelQuery(value);
    setModelPage(1);
  }, []);

  const handleModelUpstreamFilterChange = React.useCallback((value: string) => {
    setModelUpstreamFilter(value);
    setModelPage(1);
  }, []);

  const handleModelVendorFilterChange = React.useCallback((value: string) => {
    setModelVendorFilter(value);
    setModelPage(1);
  }, []);

  const handleModelProtocolFilterChange = React.useCallback((value: string) => {
    setModelProtocolFilter(value);
    setModelPage(1);
  }, []);

  const handleUserQueryChange = React.useCallback((value: string) => {
    setUserQuery(value);
    setUserPage(1);
  }, []);

  const handleUserSubscriptionFilterChange = React.useCallback((value: string) => {
    setUserSubscriptionFilter(value);
    setUserPage(1);
  }, []);

  const handleUserIdentityFilterChange = React.useCallback((value: string) => {
    setUserIdentityFilter(value);
    setUserPage(1);
  }, []);

  const handleModelPageSizeChange = React.useCallback((value: number) => {
    setModelPageSizeState(value);
    setModelPage(1);
  }, []);

  const handleUserPageSizeChange = React.useCallback((value: number) => {
    setUserPageSizeState(value);
    setUserPage(1);
  }, []);

  const refreshModels = React.useCallback(() => {
    setModelReloadKey((current) => current + 1);
  }, []);

  const refreshUsers = React.useCallback(() => {
    setUserReloadKey((current) => current + 1);
  }, []);

  const selectAllModels = React.useCallback(async () => {
    setModelBulkLoading(true);
    try {
      const token = await resolveAccessToken();
      const rows = await listAllAdminPages((options) =>
        listAdminLLMModels(token, {
          ...options,
          query: modelQuery.trim(),
          upstream: modelUpstreamFilter,
          vendor: modelVendorFilter,
          protocol: modelProtocolFilter,
        }),
      );
      setModelIDs((current) => {
        const next = new Set(current);
        rows.forEach((model) => next.add(model.id));
        return next;
      });
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
    } finally {
      setModelBulkLoading(false);
    }
  }, [modelProtocolFilter, modelQuery, modelUpstreamFilter, modelVendorFilter, t]);

  const selectAllUsers = React.useCallback(async () => {
    setUserBulkLoading(true);
    try {
      const token = await resolveAccessToken();
      const rows = await listAllAdminPages((options) =>
        listAdminUsers(token, {
          ...options,
          query: userQuery.trim(),
          subscriptionStatus: userSubscriptionFilter,
          identityProvider: userIdentityFilter,
        }),
      );
      setUserIDs((current) => {
        const next = new Set(current);
        rows.forEach((user) => next.add(user.id));
        return next;
      });
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
    } finally {
      setUserBulkLoading(false);
    }
  }, [t, userIdentityFilter, userQuery, userSubscriptionFilter]);

  const clearModelSelection = React.useCallback(() => {
    setModelIDs(new Set());
  }, []);

  const clearUserSelection = React.useCallback(() => {
    setUserIDs(new Set());
  }, []);

  const handleSave = React.useCallback(async () => {
    if (!group) {
      return;
    }
    setSaving(true);
    let shouldRefreshGroups = false;
    let shouldInvalidateReferenceData = false;
    try {
      const token = await resolveAccessToken();
      const parsed = parseFloat(rateMultiplier);
      const rateMultiplierPercent =
        Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 100) : 100;
      await updatePermissionGroup(token, group.id, { name, description, rateMultiplierPercent });
      shouldRefreshGroups = true;
      await setGroupModels(token, group.id, Array.from(modelIDs), modelRules);
      shouldInvalidateReferenceData = true;
      if (!group.isDefault) {
        await setGroupUsers(token, group.id, Array.from(userIDs));
      }
      toast.success(t("saved"));
      invalidateAdminReferenceDataCache();
      onOpenChange(false);
      await onSaved();
    } catch (error) {
      if (shouldInvalidateReferenceData) {
        invalidateAdminReferenceDataCache();
      }
      if (shouldRefreshGroups) {
        void onSaved();
      }
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    } finally {
      setSaving(false);
    }
  }, [description, group, modelIDs, modelRules, name, rateMultiplier, onOpenChange, onSaved, t, userIDs]);

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
          ...KNOWN_VENDOR_OPTIONS.map(({ label, value }) => ({ label, value })),
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
