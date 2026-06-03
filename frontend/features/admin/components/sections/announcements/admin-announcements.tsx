"use client";

import * as React from "react";
import { useLocale, useTranslations } from "next-intl";
import { Edit3, Pin, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableRow,
  TableSkeletonRows,
} from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { StreamdownRender } from "@/features/chat/components/markdown/streamdown-render";
import { AdminDateRangeFilter } from "@/features/admin/components/admin-date-range-filter";
import { adminDateTimeFormValue, adminDateTimeValueToISOString } from "@/features/admin/components/admin-date-time-picker";
import {
  createAdminAnnouncement,
  deleteAdminAnnouncement,
  listAdminAnnouncements,
  updateAdminAnnouncement,
} from "@/features/admin/api";
import type {
  AdminAnnouncementDTO,
  CreateAdminAnnouncementRequest,
  UpdateAdminAnnouncementRequest,
} from "@/features/admin/api/announcements.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { cn } from "@/lib/utils";

type AnnouncementForm = {
  id?: number;
  title: string;
  contentMarkdown: string;
  status: "active" | "inactive";
  type: "critical" | "warning" | "info" | "normal" | "general";
  pinned: boolean;
  priority: string;
  startsAt: string;
  expiresAt: string;
};

const emptyForm: AnnouncementForm = {
  title: "",
  contentMarkdown: "",
  status: "active",
  type: "general",
  pinned: false,
  priority: "0",
  startsAt: "",
  expiresAt: "",
};

function toDateRangeValue(value: string): string {
  const [dateText] = value.trim().split("T");
  return /^\d{4}-\d{2}-\d{2}$/.test(dateText) ? dateText : "";
}

function dateRangeBoundaryValue(value: string, boundary: "start" | "end"): string {
  if (!value.trim()) {
    return "";
  }
  return `${value}T${boundary === "start" ? "00:00:00" : "23:59:59"}`;
}

function formFromAnnouncement(item: AdminAnnouncementDTO): AnnouncementForm {
  return {
    id: item.id,
    title: item.title,
    contentMarkdown: item.contentMarkdown,
    status: item.status === "inactive" ? "inactive" : "active",
    type: normalizeAnnouncementType(item.type),
    pinned: Boolean(item.pinned),
    priority: String(item.priority ?? 0),
    startsAt: adminDateTimeFormValue(item.startsAt),
    expiresAt: adminDateTimeFormValue(item.expiresAt),
  };
}

function formatDateTime(value: string | null, locale: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function activeWindowLabel(item: AdminAnnouncementDTO, locale: string): string {
  const start = formatDateTime(item.startsAt, locale);
  const end = formatDateTime(item.expiresAt, locale);
  if (start === "-" && end === "-") {
    return "-";
  }
  return `${start} / ${end}`;
}

function isCurrentlyVisible(item: AdminAnnouncementDTO): boolean {
  const now = Date.now();
  const startsAt = item.startsAt ? new Date(item.startsAt).getTime() : null;
  const expiresAt = item.expiresAt ? new Date(item.expiresAt).getTime() : null;
  return item.status === "active" &&
    (startsAt === null || startsAt <= now) &&
    (expiresAt === null || expiresAt > now);
}

function payloadFromForm(form: AnnouncementForm): CreateAdminAnnouncementRequest {
  return {
    title: form.title.trim(),
    contentMarkdown: form.contentMarkdown.trim(),
    status: form.status,
    type: form.type,
    pinned: form.pinned,
    priority: Number.parseInt(form.priority, 10) || 0,
    startsAt: adminDateTimeValueToISOString(form.startsAt) ?? null,
    expiresAt: adminDateTimeValueToISOString(form.expiresAt) ?? null,
  };
}

function normalizeAnnouncementType(value: string): AnnouncementForm["type"] {
  switch (value) {
    case "critical":
    case "warning":
    case "info":
    case "normal":
    case "general":
      return value;
    default:
      return "general";
  }
}

function announcementTypeClassName(value: string): string {
  switch (value) {
    case "critical":
      return "border-red-500/30 bg-red-50 text-red-700 dark:bg-red-500/10 dark:text-red-300";
    case "warning":
      return "border-yellow-500/30 bg-yellow-50 text-yellow-700 dark:bg-yellow-500/10 dark:text-yellow-300";
    case "info":
      return "border-blue-500/30 bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-300";
    case "normal":
      return "border-emerald-500/30 bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300";
    default:
      return "border-border bg-background text-muted-foreground";
  }
}

export function AdminAnnouncementsPage() {
  const t = useTranslations("adminAnnouncements");
  const common = useTranslations("common");
  const locale = useLocale();
  const { accessToken } = useAuthSession();
  const [items, setItems] = React.useState<AdminAnnouncementDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(25);
  const [query, setQuery] = React.useState("");
  const [status, setStatus] = React.useState("");
  const [typeFilter, setTypeFilter] = React.useState("");
  const [pinnedFilter, setPinnedFilter] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<AnnouncementForm>(emptyForm);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<AdminAnnouncementDTO | null>(null);
  const [priorityDrafts, setPriorityDrafts] = React.useState<Record<number, string>>({});

  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const data = await listAdminAnnouncements(accessToken, { page, pageSize, query, status, type: typeFilter, pinned: pinnedFilter });
      setItems(data.results);
      setTotal(data.total);
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [accessToken, page, pageSize, pinnedFilter, query, status, t, typeFilter]);

  React.useEffect(() => {
    void load();
  }, [load]);

  function openCreate() {
    setForm(emptyForm);
    setDialogOpen(true);
  }

  function openEdit(item: AdminAnnouncementDTO) {
    setForm(formFromAnnouncement(item));
    setDialogOpen(true);
  }

  async function save() {
    const payload = payloadFromForm(form);
    if (!payload.title || !payload.contentMarkdown) {
      toast.error(t("toast.invalid"));
      return;
    }
    setSaving(true);
    try {
      if (form.id) {
        const updatePayload: UpdateAdminAnnouncementRequest = payload;
        const data = await updateAdminAnnouncement(accessToken, form.id, updatePayload);
        setItems((current) => current.map((item) => item.id === data.announcement.id ? data.announcement : item));
        toast.success(t("toast.updated"));
      } else {
        const data = await createAdminAnnouncement(accessToken, payload);
        setItems((current) => [data.announcement, ...current].slice(0, pageSize));
        setTotal((current) => current + 1);
        toast.success(t("toast.created"));
      }
      setDialogOpen(false);
    } catch (error) {
      toast.error(form.id ? t("toast.updateFailed") : t("toast.createFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function toggleStatus(item: AdminAnnouncementDTO, checked: boolean) {
    const nextStatus = checked ? "active" : "inactive";
    setItems((current) => current.map((row) => row.id === item.id ? { ...row, status: nextStatus } : row));
    try {
      const data = await updateAdminAnnouncement(accessToken, item.id, { status: nextStatus });
      setItems((current) => current.map((row) => row.id === item.id ? data.announcement : row));
    } catch (error) {
      setItems((current) => current.map((row) => row.id === item.id ? item : row));
      toast.error(t("toast.statusFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function togglePinned(item: AdminAnnouncementDTO, checked: boolean) {
    setItems((current) => current.map((row) => row.id === item.id ? { ...row, pinned: checked } : row));
    try {
      const data = await updateAdminAnnouncement(accessToken, item.id, { pinned: checked });
      setItems((current) => current.map((row) => row.id === item.id ? data.announcement : row));
    } catch (error) {
      setItems((current) => current.map((row) => row.id === item.id ? item : row));
      toast.error(t("toast.updateFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function updateType(item: AdminAnnouncementDTO, value: string) {
    const nextType = normalizeAnnouncementType(value);
    if (nextType === normalizeAnnouncementType(item.type)) {
      return;
    }
    setItems((current) => current.map((row) => row.id === item.id ? { ...row, type: nextType } : row));
    try {
      const data = await updateAdminAnnouncement(accessToken, item.id, { type: nextType });
      setItems((current) => current.map((row) => row.id === item.id ? data.announcement : row));
    } catch (error) {
      setItems((current) => current.map((row) => row.id === item.id ? item : row));
      toast.error(t("toast.updateFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  function setPriorityDraft(id: number, value: string) {
    setPriorityDrafts((current) => ({ ...current, [id]: value }));
  }

  function clearPriorityDraft(id: number) {
    setPriorityDrafts((current) => {
      const next = { ...current };
      delete next[id];
      return next;
    });
  }

  async function commitPriority(item: AdminAnnouncementDTO) {
    const draft = priorityDrafts[item.id];
    if (draft === undefined) {
      return;
    }
    const trimmed = draft.trim();
    if (!trimmed) {
      clearPriorityDraft(item.id);
      return;
    }
    const nextPriority = Number.parseInt(trimmed, 10);
    if (!Number.isFinite(nextPriority)) {
      clearPriorityDraft(item.id);
      toast.error(t("toast.priorityInvalid"));
      return;
    }
    if (nextPriority === item.priority) {
      clearPriorityDraft(item.id);
      return;
    }

    clearPriorityDraft(item.id);
    setItems((current) => current.map((row) => row.id === item.id ? { ...row, priority: nextPriority } : row));
    try {
      const data = await updateAdminAnnouncement(accessToken, item.id, { priority: nextPriority });
      setItems((current) => current.map((row) => row.id === item.id ? data.announcement : row));
    } catch (error) {
      setItems((current) => current.map((row) => row.id === item.id ? item : row));
      toast.error(t("toast.updateFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  function handlePriorityKeyDown(event: React.KeyboardEvent<HTMLInputElement>, item: AdminAnnouncementDTO) {
    if (event.key === "Enter") {
      event.currentTarget.blur();
      return;
    }
    if (event.key === "Escape") {
      clearPriorityDraft(item.id);
      event.currentTarget.blur();
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) {
      return;
    }
    const target = deleteTarget;
    setSaving(true);
    try {
      await deleteAdminAnnouncement(accessToken, target.id);
      setItems((current) => current.filter((item) => item.id !== target.id));
      setTotal((current) => Math.max(0, current - 1));
      setDeleteTarget(null);
      toast.success(t("toast.deleted"));
    } catch (error) {
      toast.error(t("toast.deleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-3 pb-10">
      <div className="flex h-10 items-center px-1">
        <h3 className="text-sm font-semibold">{t("title")}</h3>
      </div>

      <div className="space-y-3">
        <TableToolbar
          query={query}
          onQueryChange={(value) => {
            setQuery(value);
            setPage(1);
          }}
          queryPlaceholder={t("searchPlaceholder")}
          filters={[
            {
              key: "status",
              label: t("statusFilter"),
              value: status,
              onValueChange: (value) => {
                setStatus(value);
                setPage(1);
              },
              options: [
                { value: "", label: t("allStatuses") },
                { value: "active", label: t("status.active") },
                { value: "inactive", label: t("status.inactive") },
              ],
            },
            {
              key: "type",
              label: t("typeFilter"),
              value: typeFilter,
              onValueChange: (value) => {
                setTypeFilter(value);
                setPage(1);
              },
              options: [
                { value: "", label: t("allTypes") },
                { value: "general", label: t("types.general") },
                { value: "normal", label: t("types.normal") },
                { value: "info", label: t("types.info") },
                { value: "warning", label: t("types.warning") },
                { value: "critical", label: t("types.critical") },
              ],
            },
            {
              key: "pinned",
              label: t("pinnedFilter"),
              value: pinnedFilter,
              onValueChange: (value) => {
                setPinnedFilter(value);
                setPage(1);
              },
              options: [
                { value: "", label: t("allPinned") },
                { value: "true", label: t("pinned.yes") },
                { value: "false", label: t("pinned.no") },
              ],
            },
          ]}
          loading={loading || saving}
          onRefresh={load}
        >
          <Button type="button" size="sm" onClick={openCreate} disabled={loading || saving}>
            <Plus className="size-3.5 stroke-1" />
            {t("create")}
          </Button>
        </TableToolbar>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[260px]">{t("columns.title")}</TableHead>
              <TableHead className="w-[86px]">{t("columns.type")}</TableHead>
              <TableHead className="w-[72px] text-center">{t("columns.pinned")}</TableHead>
              <TableHead className="w-[86px] text-center">{t("columns.status")}</TableHead>
              <TableHead className="w-[90px]">{t("columns.priority")}</TableHead>
              <TableHead className="w-[190px]">{t("columns.window")}</TableHead>
              <TableHead stickyEnd className="w-[92px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? <TableSkeletonRows colSpan={7} rowCount={5} /> : null}
            {!loading && items.length === 0 ? <TableEmptyRow colSpan={7}>{t("empty")}</TableEmptyRow> : null}
            {!loading && items.map((item) => {
              const visible = isCurrentlyVisible(item);
              return (
                <TableRow key={item.id} tone={!visible ? "muted" : undefined} className={cn(!visible && "text-muted-foreground")}>
                  <TableCell className="max-w-[360px] py-2">
                    <div className="min-w-0">
                      <p className="flex min-w-0 items-center gap-1.5 truncate font-medium text-foreground">
                        {item.pinned ? <Pin className="size-3 shrink-0 text-primary" /> : null}
                        <span className="min-w-0 truncate">{item.title}</span>
                      </p>
                      <p className="truncate text-[11px] text-muted-foreground">{item.contentMarkdown}</p>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Select
                      value={normalizeAnnouncementType(item.type)}
                      onValueChange={(value) => void updateType(item, value)}
                      disabled={saving}
                    >
                      <SelectTrigger size="xs" className={cn("h-7 w-[82px] px-2 text-[11px]", announcementTypeClassName(item.type))}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent position="popper" align="center" className="z-[100]">
                        <SelectItem value="general" className="text-[11px]">{t("types.general")}</SelectItem>
                        <SelectItem value="normal" className="text-[11px]">{t("types.normal")}</SelectItem>
                        <SelectItem value="info" className="text-[11px]">{t("types.info")}</SelectItem>
                        <SelectItem value="warning" className="text-[11px]">{t("types.warning")}</SelectItem>
                        <SelectItem value="critical" className="text-[11px]">{t("types.critical")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell className="text-center">
                    <div className="flex justify-center">
                      <Switch
                        size="sm"
                        checked={Boolean(item.pinned)}
                        onCheckedChange={(checked) => void togglePinned(item, checked)}
                        disabled={saving}
                        aria-label={t("fields.pinned")}
                      />
                    </div>
                  </TableCell>
                  <TableCell className="text-center">
                    <div className="flex justify-center">
                      <Switch
                        size="sm"
                        checked={item.status === "active"}
                        onCheckedChange={(checked) => void toggleStatus(item, checked)}
                        disabled={saving}
                        aria-label={item.status === "active" ? t("disable") : t("enable")}
                      />
                    </div>
                  </TableCell>
                  <TableCell>
                    <Input
                      type="text"
                      inputMode="numeric"
                      value={priorityDrafts[item.id] ?? String(item.priority)}
                      onChange={(event) => setPriorityDraft(item.id, event.target.value)}
                      onBlur={() => void commitPriority(item)}
                      onKeyDown={(event) => handlePriorityKeyDown(event, item)}
                      disabled={saving}
                      aria-label={t("fields.priority")}
                      className="h-7 w-[58px] px-2 text-left text-xs tabular-nums"
                    />
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{activeWindowLabel(item, locale)}</TableCell>
                  <TableCell stickyEnd className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button type="button" size="icon-sm" variant="ghost" onClick={() => openEdit(item)} aria-label={t("edit")}>
                        <Edit3 className="size-3.5 stroke-1" />
                      </Button>
                      <Button type="button" size="icon-sm" variant="ghost" className="text-destructive hover:bg-destructive/10 hover:text-destructive" onClick={() => setDeleteTarget(item)} aria-label={t("delete")}>
                        <Trash2 className="size-3.5 stroke-1" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>

        <TablePagination
          total={total}
          page={page}
          pageCount={pageCount}
          pageSize={pageSize}
          onPageChange={setPage}
          onPageSizeChange={(nextSize) => {
            setPageSize(nextSize);
            setPage(1);
          }}
          loading={loading}
        />
      </div>

      <Dialog open={dialogOpen} onOpenChange={(nextOpen) => !saving && setDialogOpen(nextOpen)}>
        <DialogContent className="sm:max-w-[920px]">
          <DialogHeader>
            <DialogTitle>{form.id ? t("editTitle") : t("createTitle")}</DialogTitle>
            <DialogDescription>{t("dialogDescription")}</DialogDescription>
          </DialogHeader>

          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              void save();
            }}
          >
            <div className="grid grid-cols-2 gap-5">
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.title")}</p>
                <Input value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} disabled={saving} />
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.timeRange")}</p>
                <AdminDateRangeFilter
                  fromValue={toDateRangeValue(form.startsAt)}
                  toValue={toDateRangeValue(form.expiresAt)}
                  disabled={saving}
                  placeholder={t("fields.alwaysActive")}
                  onFromChange={(value) => setForm((current) => ({ ...current, startsAt: dateRangeBoundaryValue(value, "start") }))}
                  onToChange={(value) => setForm((current) => ({ ...current, expiresAt: dateRangeBoundaryValue(value, "end") }))}
                />
              </div>

              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.type")}</p>
                <Select value={form.type} onValueChange={(value) => setForm({ ...form, type: normalizeAnnouncementType(value) })} disabled={saving}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="general">{t("types.general")}</SelectItem>
                    <SelectItem value="normal">{t("types.normal")}</SelectItem>
                    <SelectItem value="info">{t("types.info")}</SelectItem>
                    <SelectItem value="warning">{t("types.warning")}</SelectItem>
                    <SelectItem value="critical">{t("types.critical")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.priority")}</p>
                <Input type="number" value={form.priority} onChange={(event) => setForm({ ...form, priority: event.target.value })} disabled={saving} />
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.pinned")}</p>
                <div className="flex h-9 items-center">
                  <Switch
                    size="sm"
                    checked={form.pinned}
                    onCheckedChange={(checked) => setForm({ ...form, pinned: checked })}
                    disabled={saving}
                    aria-label={t("fields.pinned")}
                  />
                </div>
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.status")}</p>
                <div className="flex h-9 items-center">
                  <Switch
                    size="sm"
                    checked={form.status === "active"}
                    onCheckedChange={(checked) => setForm({ ...form, status: checked ? "active" : "inactive" })}
                    disabled={saving}
                    aria-label={t("fields.status")}
                  />
                </div>
              </div>

              <div className="col-span-2 space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.contentMarkdown")}</p>
                <Textarea
                  value={form.contentMarkdown}
                  onChange={(event) => setForm({ ...form, contentMarkdown: event.target.value })}
                  disabled={saving}
                  className="h-32 resize-none overflow-y-auto text-xs [field-sizing:fixed]"
                />
              </div>

              <div className="col-span-2 space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.preview")}</p>
                <div className="h-32 overflow-y-auto rounded-md border border-border/60 bg-muted/20 px-3 py-2">
                  <StreamdownRender content={form.contentMarkdown || t("previewEmpty")} className="text-sm" />
                </div>
              </div>
            </div>

            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setDialogOpen(false)} disabled={saving}>
                {common("actions.cancel")}
              </Button>
              <Button type="submit" disabled={saving}>
                {saving ? <SpinnerLabel>{common("actions.saving")}</SpinnerLabel> : common("actions.save")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={Boolean(deleteTarget)} onOpenChange={(open) => !saving && !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>
              {common("actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={(event) => {
                event.preventDefault();
                void confirmDelete();
              }}
              disabled={saving}
            >
              {saving ? t("deleting") : t("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
