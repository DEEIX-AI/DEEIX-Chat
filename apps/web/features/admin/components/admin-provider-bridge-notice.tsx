"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

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
import { useAdminProviderBridgeNotice } from "@/features/admin/hooks/use-admin-provider-bridge-notice";

/**
 * Warns administrators once per session when identity providers are configured
 * but the server cannot run the OAuth handoff because PUBLIC_API_BASE_URL is
 * missing. Sign-in through those providers is unavailable until it is set.
 */
export function AdminProviderBridgeNotice({ basePath }: { basePath: string }) {
  const t = useTranslations("adminLogin.bridgeNotice");
  const router = useRouter();
  const { open, setOpen, dismiss } = useAdminProviderBridgeNotice();

  return (
    <AlertDialog open={open} onOpenChange={(next) => (next ? setOpen(true) : dismiss())}>
      <AlertDialogContent size="sm">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("title")}</AlertDialogTitle>
          <AlertDialogDescription>{t("description")}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={dismiss}>{t("dismiss")}</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              dismiss();
              router.push(`${basePath}/login`);
            }}
          >
            {t("openSettings")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
