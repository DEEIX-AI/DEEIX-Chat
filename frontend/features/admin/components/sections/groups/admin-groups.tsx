"use client";

import * as React from "react";
import { Plus, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SettingsSection } from "@/shared/components/settings-layout";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { listAllAdminPages } from "@/features/admin/api/shared";
import { listAdminLLMModels } from "@/features/admin/api/llm";
import { listAdminUsers } from "@/features/admin/api/accounts";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import type { UserDTO } from "@/shared/api/auth.types";
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
} from "@/features/admin/api/permission-groups";

type GroupCounts = Record<number, { models: number; users: number }>;

export function AdminGroupsPage() {
  const t = useTranslations("adminGroups");
  const [groups, setGroups] = React.useState<PermissionGroup[]>([]);
  const [counts, setCounts] = React.useState<GroupCounts>({});
  const [allModels, setAllModels] = React.useState<AdminLLMModelDTO[]>([]);
  const [allUsers, setAllUsers] = React.useState<UserDTO[]>([]);
  const [loading, setLoading] = React.useState(true);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<PermissionGroup | null>(null);
  const [deleting, setDeleting] = React.useState<PermissionGroup | null>(null);

  const loadGroups = React.useCallback(async () => {
    const token = await resolveAccessToken();
    const list = await listPermissionGroups(token);
    setGroups(list);
    const entries = await Promise.all(
      list.map(async (group) => {
        const [models, users] = await Promise.all([
          listGroupModels(token, group.id),
          listGroupUsers(token, group.id),
        ]);
        return [group.id, { models: models.length, users: users.length }] as const;
      }),
    );
    setCounts(Object.fromEntries(entries));
  }, []);

  React.useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await resolveAccessToken();
        const [models, users] = await Promise.all([
          listAllAdminPages((options) => listAdminLLMModels(token, options)),
          listAllAdminPages((options) => listAdminUsers(token, options)),
        ]);
        if (cancelled) {
          return;
        }
        setAllModels(models);
        setAllUsers(users);
        await loadGroups();
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
  }, [loadGroups, t]);

  const handleDelete = React.useCallback(async () => {
    if (!deleting) {
      return;
    }
    try {
      const token = await resolveAccessToken();
      await deletePermissionGroup(token, deleting.id);
      toast.success(t("deleted"));
      setDeleting(null);
      await loadGroups();
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    }
  }, [deleting, loadGroups, t]);

  return (
    <SettingsSection
      title={t("title")}
      className="px-1"
      actions={
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          <Plus className="size-4" />
          {t("createGroup")}
        </Button>
      }
    >
      <p className="text-sm text-muted-foreground">{t("description")}</p>

      {loading ? (
        <SpinnerLabel />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("code")}</TableHead>
              <TableHead>{t("name")}</TableHead>
              <TableHead>{t("descriptionField")}</TableHead>
              <TableHead className="text-right">{t("modelCount")}</TableHead>
              <TableHead className="text-right">{t("memberCount")}</TableHead>
              <TableHead className="w-16" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {groups.length === 0 ? (
              <TableEmptyRow colSpan={6}>{t("noGroups")}</TableEmptyRow>
            ) : (
              groups.map((group) => (
                <TableRow
                  key={group.id}
                  className="cursor-pointer"
                  onClick={() => setEditing(group)}
                >
                  <TableCell className="font-mono text-xs">
                    {group.code}
                    {group.isDefault ? (
                      <Badge variant="secondary" className="ml-2">
                        {t("default")}
                      </Badge>
                    ) : null}
                  </TableCell>
                  <TableCell>{group.name}</TableCell>
                  <TableCell className="max-w-xs truncate text-muted-foreground">
                    {group.description}
                  </TableCell>
                  <TableCell className="text-right">{counts[group.id]?.models ?? 0}</TableCell>
                  <TableCell className="text-right">{counts[group.id]?.users ?? 0}</TableCell>
                  <TableCell className="text-right">
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
            )}
          </TableBody>
        </Table>
      )}

      <CreateGroupDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={loadGroups}
      />

      <GroupEditSheet
        group={editing}
        allModels={allModels}
        allUsers={allUsers}
        onOpenChange={(open) => {
          if (!open) {
            setEditing(null);
          }
        }}
        onSaved={loadGroups}
      />

      <AlertDialog open={deleting !== null} onOpenChange={(open) => (!open ? setDeleting(null) : null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("confirmDeleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("confirmDelete")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete}>{t("deleteGroup")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
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
  const [code, setCode] = React.useState("");
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (open) {
      setCode("");
      setName("");
      setDescription("");
    }
  }, [open]);

  const handleCreate = React.useCallback(async () => {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      await createPermissionGroup(token, { code, name, description });
      toast.success(t("created"));
      onOpenChange(false);
      await onCreated();
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    } finally {
      setSaving(false);
    }
  }, [code, description, name, onCreated, onOpenChange, t]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("createGroup")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="group-code">{t("code")}</Label>
            <Input
              id="group-code"
              value={code}
              onChange={(event) => setCode(event.target.value)}
              placeholder="team-a"
            />
            <p className="text-xs text-muted-foreground">{t("codeHint")}</p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="group-name">{t("name")}</Label>
            <Input id="group-name" value={name} onChange={(event) => setName(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="group-desc">{t("descriptionField")}</Label>
            <Textarea
              id="group-desc"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            {t("cancel")}
          </Button>
          <Button onClick={handleCreate} disabled={saving || !code.trim() || !name.trim()}>
            {t("create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function GroupEditSheet({
  group,
  allModels,
  allUsers,
  onOpenChange,
  onSaved,
}: {
  group: PermissionGroup | null;
  allModels: AdminLLMModelDTO[];
  allUsers: UserDTO[];
  onOpenChange: (open: boolean) => void;
  onSaved: () => Promise<void>;
}) {
  const t = useTranslations("adminGroups");
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [modelIDs, setModelIDs] = React.useState<Set<number>>(new Set());
  const [userIDs, setUserIDs] = React.useState<Set<number>>(new Set());
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (!group) {
      return;
    }
    setName(group.name);
    setDescription(group.description);
    let cancelled = false;
    (async () => {
      try {
        const token = await resolveAccessToken();
        const [models, users] = await Promise.all([
          listGroupModels(token, group.id),
          listGroupUsers(token, group.id),
        ]);
        if (!cancelled) {
          setModelIDs(new Set(models));
          setUserIDs(new Set(users));
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [group, t]);

  const toggle = React.useCallback((setter: React.Dispatch<React.SetStateAction<Set<number>>>, id: number) => {
    setter((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const handleSave = React.useCallback(async () => {
    if (!group) {
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      await updatePermissionGroup(token, group.id, { name, description });
      await setGroupModels(token, group.id, Array.from(modelIDs));
      await setGroupUsers(token, group.id, Array.from(userIDs));
      toast.success(t("saved"));
      onOpenChange(false);
      await onSaved();
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    } finally {
      setSaving(false);
    }
  }, [description, group, modelIDs, name, onOpenChange, onSaved, t, userIDs]);

  return (
    <Sheet open={group !== null} onOpenChange={onOpenChange}>
      <SheetContent className="flex w-full flex-col gap-0 sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{t("editGroup")}</SheetTitle>
          <SheetDescription>{group?.code}</SheetDescription>
        </SheetHeader>
        <div className="flex-1 space-y-4 overflow-y-auto px-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="edit-name">{t("name")}</Label>
            <Input id="edit-name" value={name} onChange={(event) => setName(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="edit-desc">{t("descriptionField")}</Label>
            <Textarea
              id="edit-desc"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
          </div>
          <Tabs defaultValue="models">
            <TabsList>
              <TabsTrigger value="models">{t("models")}</TabsTrigger>
              <TabsTrigger value="users">{t("users")}</TabsTrigger>
            </TabsList>
            <TabsContent value="models" className="space-y-1">
              {allModels.length === 0 ? (
                <p className="py-4 text-sm text-muted-foreground">{t("noModels")}</p>
              ) : (
                allModels.map((model) => (
                  <label
                    key={model.id}
                    className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50"
                  >
                    <Checkbox
                      checked={modelIDs.has(model.id)}
                      onCheckedChange={() => toggle(setModelIDs, model.id)}
                    />
                    <span className="text-sm">{model.platformModelName}</span>
                  </label>
                ))
              )}
            </TabsContent>
            <TabsContent value="users" className="space-y-1">
              {allUsers.length === 0 ? (
                <p className="py-4 text-sm text-muted-foreground">{t("noUsers")}</p>
              ) : (
                allUsers.map((user) => (
                  <label
                    key={user.id}
                    className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50"
                  >
                    <Checkbox
                      checked={userIDs.has(user.id)}
                      onCheckedChange={() => toggle(setUserIDs, user.id)}
                    />
                    <span className="text-sm">{user.displayName || user.username}</span>
                    <span className="text-xs text-muted-foreground">@{user.username}</span>
                  </label>
                ))
              )}
            </TabsContent>
          </Tabs>
        </div>
        <SheetFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            {t("cancel")}
          </Button>
          <Button onClick={handleSave} disabled={saving || !name.trim()}>
            {t("save")}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
