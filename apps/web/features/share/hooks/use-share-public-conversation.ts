import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { cloneSharedConversation, getSharedConversation } from "@/shared/api/conversation";
import type { PublicSharedConversationDTO } from "@/shared/api/conversation-types";
import { fetchSharedFileContent } from "@/shared/api/file";
import { useOptionalAuthSession } from "@/shared/auth/auth-session-context";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { FileContentLoader } from "@/entities/file";

/**
 * Loads a public share snapshot and backs "continue conversation": signed-in
 * viewers clone it into their account, anonymous viewers go to login with a
 * `next` path back to this share. The page may render outside the auth
 * provider, so the session token is resolved independently as a fallback.
 */
export function useSharePublicConversation(shareID: string) {
  const t = useTranslations("share");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const router = useRouter();
  const authSession = useOptionalAuthSession();
  const [data, setData] = React.useState<PublicSharedConversationDTO | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [errorMsg, setErrorMsg] = React.useState("");
  const [resolvedAccessToken, setResolvedAccessToken] = React.useState("");
  const [cloning, setCloning] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    async function loadShare() {
      setLoading(true);
      setErrorMsg("");
      try {
        const result = await getSharedConversation(shareID);
        if (!cancelled) {
          setData(result);
        }
      } catch (error) {
        if (!cancelled) {
          setData(null);
          setErrorMsg(resolveErrorMessage(error, t("notFoundDescription")));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }
    if (shareID) {
      void loadShare();
    } else {
      setLoading(false);
      setErrorMsg(t("notFoundDescription"));
    }
    return () => {
      cancelled = true;
    };
  }, [resolveErrorMessage, shareID, t]);

  React.useEffect(() => {
    if (authSession?.accessToken) {
      setResolvedAccessToken(authSession.accessToken);
      return;
    }
    let cancelled = false;
    async function checkSession() {
      try {
        const token = await resolveAccessToken();
        if (!cancelled) {
          setResolvedAccessToken(token);
        }
      } catch {
        if (!cancelled) {
          setResolvedAccessToken("");
        }
      }
    }
    void checkSession();
    return () => {
      cancelled = true;
    };
  }, [authSession?.accessToken]);

  const loadSharedContent = React.useCallback<FileContentLoader>(
    (file, signal) => fetchSharedFileContent(shareID, file.fileID, signal),
    [shareID],
  );
  const accessToken = authSession?.accessToken || resolvedAccessToken;
  const loginNextPath = React.useMemo(() => {
    const params = new URLSearchParams();
    if (shareID) {
      params.set("conversation_id", shareID);
    }
    const nextPath = params.toString() ? `/share?${params.toString()}` : "/share";
    return `/login?next=${encodeURIComponent(nextPath)}`;
  }, [shareID]);

  const continueConversation = React.useCallback(async () => {
    if (!shareID) {
      return;
    }
    if (!accessToken) {
      router.push(loginNextPath);
      return;
    }
    setCloning(true);
    try {
      const conversation = await cloneSharedConversation(accessToken, shareID);
      router.push(`/chat?conversation_id=${encodeURIComponent(conversation.publicID)}`);
    } catch (error) {
      toast.error(t("cloneFailed"), { description: resolveErrorMessage(error, t("cloneFailed")) });
    } finally {
      setCloning(false);
    }
  }, [accessToken, loginNextPath, resolveErrorMessage, router, shareID, t]);

  return { data, loading, errorMsg, accessToken, cloning, loadSharedContent, continueConversation };
}
