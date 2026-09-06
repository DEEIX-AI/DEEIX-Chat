"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  grantConversationACL,
  listConversationACL,
  revokeConversationACL,
  type ResourceACLEntryDTO,
} from "@/shared/api/resource-acl";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useFeaturePolicy } from "@/shared/hooks/use-feature-policy";

type ConversationUserSharePanelProps = {
  conversationPublicID: string;
  enabled: boolean;
};

export function ConversationUserSharePanel({
  conversationPublicID,
  enabled,
}: ConversationUserSharePanelProps) {
  const t = useTranslations("conversation.userShare");
  const tCommon = useTranslations("common.actions");
  const featurePolicy = useFeaturePolicy();
  const [entries, setEntries] = React.useState<ResourceACLEntryDTO[]>([]);
  const [username, setUsername] = React.useState("");
  const [role, setRole] = React.useState<"viewer" | "editor">("viewer");
  const [loading, setLoading] = React.useState(false);
  const [working, setWorking] = React.useState(false);

  const refresh = React.useCallback(async () => {
    if (!enabled || !featurePolicy.resourceSharingEnabled) {
      return;
    }
    const token = await resolveAccessToken();
    if (!token) {
      return;
    }
    setLoading(true);
    try {
      setEntries(await listConversationACL(token, conversationPublicID));
    } catch {
      toast.error(t("loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [conversationPublicID, enabled, featurePolicy.resourceSharingEnabled, t]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  if (!featurePolicy.resourceSharingEnabled || !enabled) {
    return null;
  }

  const handleGrant = async () => {
    const token = await resolveAccessToken();
    if (!token || !username.trim()) {
      return;
    }
    setWorking(true);
    try {
      await grantConversationACL(token, conversationPublicID, username.trim(), role);
      setUsername("");
      await refresh();
      toast.success(t("granted"));
    } catch {
      toast.error(t("grantFailed"));
    } finally {
      setWorking(false);
    }
  };

  const handleRevoke = async (granteeUserID: number) => {
    const token = await resolveAccessToken();
    if (!token) {
      return;
    }
    setWorking(true);
    try {
      await revokeConversationACL(token, conversationPublicID, granteeUserID);
      await refresh();
      toast.success(t("revoked"));
    } catch {
      toast.error(t("revokeFailed"));
    } finally {
      setWorking(false);
    }
  };

  return (
    <div className="space-y-3 border-t border-border/50 pt-4">
      <div className="space-y-1">
        <p className="text-sm font-medium">{t("title")}</p>
        <p className="text-xs text-muted-foreground">{t("description")}</p>
      </div>
      <div className="flex flex-wrap gap-2">
        <Input
          value={username}
          onChange={(event) => setUsername(event.target.value)}
          placeholder={t("usernamePlaceholder")}
          className="min-w-[8rem] flex-1"
        />
        <Select value={role} onValueChange={(value) => setRole(value as "viewer" | "editor")}>
          <SelectTrigger className="w-[7.5rem]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="viewer">{t("roleViewer")}</SelectItem>
            <SelectItem value="editor">{t("roleEditor")}</SelectItem>
          </SelectContent>
        </Select>
        <Button type="button" disabled={working || !username.trim()} onClick={() => void handleGrant()}>
          {working ? <SpinnerLabel>{tCommon("saving")}</SpinnerLabel> : t("grant")}
        </Button>
      </div>
      <div className="space-y-2">
        {loading ? (
          <p className="text-xs text-muted-foreground">{tCommon("loading")}</p>
        ) : entries.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t("empty")}</p>
        ) : (
          entries.map((entry) => (
            <div key={entry.granteeUserID} className="flex items-center justify-between gap-2 text-xs">
              <span className="truncate">
                {entry.granteeUsername} · {entry.role === "editor" ? t("roleEditor") : t("roleViewer")}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={working}
                onClick={() => void handleRevoke(entry.granteeUserID)}
              >
                {t("revoke")}
              </Button>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
