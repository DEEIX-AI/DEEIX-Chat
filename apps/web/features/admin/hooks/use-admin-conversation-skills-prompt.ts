import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminSettingsByNamespace, patchAdminSettings } from "@/features/admin/api";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { PatchSettingItem } from "@/shared/api/settings-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/** Loads and saves the `chat.skills_prompt` setting edited on the skills tab. */
export function useAdminConversationSkillsPrompt() {
  const t = useTranslations("adminLibrary");
  const [skillsPromptValue, setSkillsPromptValue] = React.useState("");
  const [savedSkillsPromptValue, setSavedSkillsPromptValue] = React.useState("");
  const [skillsPromptLoading, setSkillsPromptLoading] = React.useState(true);
  const [skillsPromptSaving, setSkillsPromptSaving] = React.useState(false);
  const skillsPromptDirty = skillsPromptValue !== savedSkillsPromptValue;

  React.useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setSkillsPromptLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
          return;
        }
        const settings = await listAdminSettingsByNamespace(token, "chat");
        if (cancelled) {
          return;
        }
        const nextValue = settings.find((item) => item.key === "skills_prompt")?.value ?? "";
        setSkillsPromptValue(nextValue);
        setSavedSkillsPromptValue(nextValue);
      } catch (error) {
        if (!cancelled) {
          toast.error(t("toast.settingsLoadFailed"), { description: resolveAdminErrorMessage(error) });
        }
      } finally {
        if (!cancelled) {
          setSkillsPromptLoading(false);
        }
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [t]);

  const saveSkillsPrompt = React.useCallback(async () => {
    if (!skillsPromptDirty) {
      return;
    }

    setSkillsPromptSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const items: PatchSettingItem[] = [
        {
          namespace: "chat",
          key: "skills_prompt",
          value: skillsPromptValue,
        },
      ];
      const grouped = await patchAdminSettings(token, { items });
      const nextValue = grouped.chat?.find((item) => item.key === "skills_prompt")?.value ?? skillsPromptValue;
      setSkillsPromptValue(nextValue);
      setSavedSkillsPromptValue(nextValue);
      toast.success(t("toast.settingsUpdated"));
    } catch (error) {
      toast.error(t("toast.settingsSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSkillsPromptSaving(false);
    }
  }, [skillsPromptDirty, skillsPromptValue, t]);

  return {
    skillsPromptValue,
    setSkillsPromptValue,
    skillsPromptLoading,
    skillsPromptSaving,
    skillsPromptDirty,
    saveSkillsPrompt,
  };
}
