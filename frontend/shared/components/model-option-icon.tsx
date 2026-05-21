import { LobeHubIcon } from "@/shared/components/lobehub-icon";

export function ModelOptionIcon({ iconUrl, label }: { iconUrl?: string | null; label: string }) {
  return (
    <LobeHubIcon iconUrl={iconUrl} label={label} className="self-center" />
  );
}
