"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { HeightTransition } from "@/components/ui/height-transition";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { AppLogo } from "@/shared/components/app-logo";
import { CustomBrandAttribution } from "@/shared/components/powered-by-deeix";
import { isShellSessionError } from "@/shared/platform/desktop-shell";
import { ensureLocalSession } from "@/shared/platform/desktop-session";
import { commitLocalServer, commitRemoteServer, validateApiBaseUrl } from "@/shared/platform/server-address";

// Desktop first-run screen, styled as a sibling of the login page:
//   local   the bundled server, data on this machine, no account needed;
//   remote  a DEEIX Chat server you or your team operate.

const HEALTH_PATH = "/healthz";

type Step = "pick" | "remote";

export function ServerSetup({ onConfigured }: { onConfigured: () => void }) {
  const t = useTranslations("desktopSetup");
  const [step, setStep] = React.useState<Step>("pick");
  const [busy, setBusy] = React.useState(false);

  const startLocal = React.useCallback(async () => {
    setBusy(true);
    try {
      await commitLocalServer();
      await ensureLocalSession();
      onConfigured();
    } catch (error) {
      if (isAlreadyOpen(error)) {
        toast.error(t("toasts.alreadyOpen"));
      } else {
        toast.error(t("toasts.localStartFailed"), { description: describe(error) });
      }
      setBusy(false);
    }
  }, [onConfigured, t]);

  return (
    <main className="flex min-h-screen animate-in items-center justify-center px-4 py-8 text-foreground fade-in-0 duration-300">
      <div className="w-full max-w-[360px]">
        <AppLogo width={32} height={32} priority className="mx-auto h-9 w-auto" />

        <HeightTransition className="px-2">
          {step === "pick" ? (
            <div key="pick" className="animate-in pt-7 fade-in-0 slide-in-from-bottom-1 duration-300">
              <div className="space-y-2.5">
                <Button
                  type="button"
                  className="h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90"
                  disabled={busy}
                  onClick={() => void startLocal()}
                >
                  {busy ? <SpinnerLabel>{t("local.starting")}</SpinnerLabel> : t("local.action")}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  className="h-9 w-full rounded-md border-0 bg-muted text-sm font-semibold text-foreground shadow-none hover:bg-muted/80"
                  disabled={busy}
                  onClick={() => setStep("remote")}
                >
                  {t("remote.action")}
                </Button>
              </div>
            </div>
          ) : (
            <div key="remote" className="animate-in pt-7 fade-in-0 slide-in-from-bottom-1 duration-300">
              <RemoteForm labels={t} onBack={() => setStep("pick")} onConfigured={onConfigured} />
            </div>
          )}
        </HeightTransition>
      </div>

      <CustomBrandAttribution className="fixed bottom-4 right-4" />
    </main>
  );
}

function RemoteForm({
  labels,
  onBack,
  onConfigured,
}: {
  labels: ReturnType<typeof useTranslations<"desktopSetup">>;
  onBack: () => void;
  onConfigured: () => void;
}) {
  const [value, setValue] = React.useState("");
  const [busy, setBusy] = React.useState(false);

  const submit = React.useCallback(async () => {
    const candidate = validateApiBaseUrl(value);
    if (!candidate) {
      toast.error(labels("toasts.invalidUrl"));
      return;
    }
    setBusy(true);
    try {
      // Probe before pinning: a wrong address must not survive a restart.
      let response: Response;
      try {
        response = await fetch(`${candidate}${HEALTH_PATH}`, { method: "GET", cache: "no-store" });
      } catch {
        toast.error(labels("toasts.networkError"));
        return;
      }
      if (!response.ok) {
        toast.error(labels("toasts.unexpectedStatus", { status: response.status }));
        return;
      }
      await commitRemoteServer(candidate);
      onConfigured();
    } catch (error) {
      toast.error(isAlreadyOpen(error) ? labels("toasts.alreadyOpen") : labels("toasts.invalidUrl"), {
        description: isAlreadyOpen(error) ? undefined : describe(error),
      });
    } finally {
      setBusy(false);
    }
  }, [labels, onConfigured, value]);

  return (
    <form
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <div className="space-y-2">
        <label className="text-sm font-medium leading-none text-foreground" htmlFor="server-url">
          {labels("remote.title")}
        </label>
        <Input
          id="server-url"
          name="server-url"
          autoFocus
          inputMode="url"
          autoComplete="url"
          className="h-9 border-input/50"
          placeholder="https://chat.example.com"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          required
        />
      </div>
      <Button
        type="submit"
        className="mt-1 h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90"
        disabled={busy}
      >
        {busy ? <SpinnerLabel>{labels("remote.connecting")}</SpinnerLabel> : labels("remote.connect")}
      </Button>
      <Button
        type="button"
        variant="ghost"
        className="h-9 w-full text-xs text-muted-foreground shadow-none"
        disabled={busy}
        onClick={onBack}
      >
        {labels("back")}
      </Button>
    </form>
  );
}

// Rust refuses to bind a server that another tab already shows.
function isAlreadyOpen(error: unknown): boolean {
  return isShellSessionError(error) && error.kind === "tabs";
}

function describe(error: unknown): string {
  if (typeof error === "object" && error !== null && "message" in error) {
    return String((error as { message: unknown }).message);
  }
  return String(error);
}
