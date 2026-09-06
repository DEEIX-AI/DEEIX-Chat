"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  createUserAPIKey,
  listUserAPIKeys,
  revokeUserAPIKey,
  type CreateUserAPIKeyResult,
  type UserAPIKeyDTO,
} from "@/shared/api/user-api-keys";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { CopyActionButton } from "@/shared/components/copy-action";
import { SettingsSection } from "@/shared/components/settings-layout";
import { useFeaturePolicy } from "@/shared/hooks/use-feature-policy";

export function AccountAPIKeysSection() {
  const t = useTranslations("settings.accountPage.apiKeys");
  const tCommon = useTranslations("common.actions");
  const featurePolicy = useFeaturePolicy();
  const [items, setItems] = React.useState<UserAPIKeyDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [name, setName] = React.useState("");
  const [creating, setCreating] = React.useState(false);
  const [revokingID, setRevokingID] = React.useState<string | null>(null);
  const [created, setCreated] = React.useState<CreateUserAPIKeyResult | null>(null);

  const refresh = React.useCallback(async () => {
    const token = await resolveAccessToken();
    if (!token) {
      setItems([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      setItems(await listUserAPIKeys(token));
    } catch {
      toast.error(t("loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    if (!featurePolicy.userApiKeysEnabled) {
      setLoading(false);
      return;
    }
    void refresh();
  }, [featurePolicy.userApiKeysEnabled, refresh]);

  if (!featurePolicy.userApiKeysEnabled) {
    return null;
  }

  const handleCreate = async () => {
    const token = await resolveAccessToken();
    if (!token || !name.trim()) {
      return;
    }
    setCreating(true);
    try {
      const result = await createUserAPIKey(token, name.trim());
      setCreated(result);
      setName("");
      await refresh();
      toast.success(t("created"));
    } catch {
      toast.error(t("createFailed"));
    } finally {
      setCreating(false);
    }
  };

  const handleRevoke = async (publicId: string) => {
    const token = await resolveAccessToken();
    if (!token) {
      return;
    }
    setRevokingID(publicId);
    try {
      await revokeUserAPIKey(token, publicId);
      if (created?.publicId === publicId) {
        setCreated(null);
      }
      await refresh();
      toast.success(t("revoked"));
    } catch {
      toast.error(t("revokeFailed"));
    } finally {
      setRevokingID(null);
    }
  };

  return (
    <SettingsSection title={t("title")}>
      <div className="space-y-4">
        <p className="text-xs text-muted-foreground">{t("description")}</p>
        {created ? (
          <div className="space-y-2 rounded-lg border border-border/60 bg-muted/20 p-3">
            <p className="text-xs text-muted-foreground">{t("plaintextHint")}</p>
            <div className="flex items-center gap-2">
              <Input readOnly value={created.key} className="font-mono text-xs" />
              <CopyActionButton
                type="button"
                variant="outline"
                size="sm"
                value={created.key}
                messages={{ copied: t("copied"), failed: t("copyFailed") }}
              />
            </div>
          </div>
        ) : null}

        <div className="flex flex-wrap items-end gap-2">
          <div className="min-w-[12rem] flex-1 space-y-1">
            <p className="text-xs font-medium">{t("nameLabel")}</p>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("namePlaceholder")}
              maxLength={128}
            />
          </div>
          <Button type="button" disabled={creating || !name.trim()} onClick={() => void handleCreate()}>
            {creating ? <SpinnerLabel>{tCommon("saving")}</SpinnerLabel> : t("create")}
          </Button>
        </div>

        <div className="space-y-2">
          {loading ? (
            <p className="text-xs text-muted-foreground">{tCommon("loading")}</p>
          ) : items.length === 0 ? (
            <p className="text-xs text-muted-foreground">{t("empty")}</p>
          ) : (
            items.map((item) => (
              <div
                key={item.publicId}
                className="flex items-center justify-between gap-3 rounded-lg bg-muted/30 px-3 py-2"
              >
                <div className="min-w-0 space-y-0.5">
                  <p className="truncate text-xs font-medium">{item.name}</p>
                  <p className="truncate font-mono text-[11px] text-muted-foreground">
                    {item.keyPrefix}…{item.revokedAt ? ` · ${t("statusRevoked")}` : ""}
                  </p>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  disabled={Boolean(item.revokedAt) || revokingID === item.publicId}
                  onClick={() => void handleRevoke(item.publicId)}
                >
                  {revokingID === item.publicId ? <SpinnerLabel>{tCommon("saving")}</SpinnerLabel> : t("revoke")}
                </Button>
              </div>
            ))
          )}
        </div>
      </div>
    </SettingsSection>
  );
}
