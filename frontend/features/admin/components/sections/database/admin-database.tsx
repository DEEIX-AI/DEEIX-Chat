"use client";

import * as React from "react";
import { DatabaseBackup, Download, RotateCcw, Upload } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { downloadAdminDatabaseBackup, getAdminDatabaseBackupInfo, restoreAdminDatabaseBackup, type AdminDatabaseBackupInfo } from "@/features/admin/api";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { useAuthSession } from "@/shared/auth/auth-session-context";

export function AdminDatabasePage() {
  const t = useTranslations("adminDatabase");
  const { accessToken, user } = useAuthSession();
  const inputRef = React.useRef<HTMLInputElement>(null);
  const [info, setInfo] = React.useState<AdminDatabaseBackupInfo | null>(null);
  const [selectedFile, setSelectedFile] = React.useState<File | null>(null);
  const [confirmOpen, setConfirmOpen] = React.useState(false);
  const [downloading, setDownloading] = React.useState(false);
  const [restoring, setRestoring] = React.useState(false);

  React.useEffect(() => {
    if (!accessToken || user?.role !== "superadmin") return;
    void getAdminDatabaseBackupInfo(accessToken).then(setInfo).catch(() => undefined);
  }, [accessToken, user?.role]);

  if (user?.role !== "superadmin") return <div className="rounded-xl border p-6 text-sm text-muted-foreground">{t("superadminOnly")}</div>;

  async function handleDownload() {
    if (!accessToken || downloading) return;
    setDownloading(true);
    try { await downloadAdminDatabaseBackup(accessToken); toast.success(t("backup.success")); }
    catch (error) { toast.error(resolveAdminErrorMessage(error, t("backup.failed"))); }
    finally { setDownloading(false); }
  }

  async function handleRestore() {
    if (!accessToken || !selectedFile || restoring) return;
    setRestoring(true);
    try {
      await restoreAdminDatabaseBackup(accessToken, selectedFile);
      toast.success(t("restore.success"));
      setSelectedFile(null); setConfirmOpen(false);
      if (inputRef.current) inputRef.current.value = "";
    } catch (error) { toast.error(resolveAdminErrorMessage(error, t("restore.failed"))); }
    finally { setRestoring(false); }
  }

  const extension = info?.driver === "sqlite" ? ".sqlite3" : ".dump";
  return (
    <div className="space-y-6 pb-8">
      <div><h2 className="text-xl font-semibold tracking-tight">{t("title")}</h2><p className="mt-1 text-sm text-muted-foreground">{t("description")}</p></div>
      <div className="grid gap-4 md:grid-cols-2">
        <Card className="gap-4"><CardHeader><div className="mb-2 flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary"><DatabaseBackup className="size-5" /></div><CardTitle>{t("backup.title")}</CardTitle><CardDescription>{t("backup.description", { extension })}</CardDescription></CardHeader><CardContent><Button onClick={() => void handleDownload()} disabled={downloading || !info}><Download className="size-4" />{downloading ? t("backup.downloading") : t("backup.action")}</Button></CardContent></Card>
        <Card className="gap-4 border-destructive/25"><CardHeader><div className="mb-2 flex size-10 items-center justify-center rounded-lg bg-destructive/10 text-destructive"><RotateCcw className="size-5" /></div><CardTitle>{t("restore.title")}</CardTitle><CardDescription>{t("restore.description", { extension })}</CardDescription></CardHeader><CardContent className="space-y-3"><input ref={inputRef} type="file" accept={extension} className="block w-full text-sm file:mr-3 file:rounded-md file:border-0 file:bg-muted file:px-3 file:py-2 file:text-sm file:font-medium" onChange={(event) => setSelectedFile(event.target.files?.[0] ?? null)} /><Button variant="destructive" disabled={!selectedFile || restoring} onClick={() => setConfirmOpen(true)}><Upload className="size-4" />{restoring ? t("restore.restoring") : t("restore.action")}</Button></CardContent></Card>
      </div>
      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{t("restore.confirmTitle")}</AlertDialogTitle><AlertDialogDescription>{t("restore.confirmDescription", { filename: selectedFile?.name ?? "" })}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={restoring}>{t("cancel")}</AlertDialogCancel><AlertDialogAction variant="destructive" disabled={restoring} onClick={(event) => { event.preventDefault(); void handleRestore(); }}>{restoring ? t("restore.restoring") : t("restore.confirmAction")}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
    </div>
  );
}
