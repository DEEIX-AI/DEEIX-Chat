import { ModelIcon } from "@/shared/components/model-icon";

export function ModelOptionIcon({ iconUrl, label }: { iconUrl?: string | null; label: string }) {
  return (
    <ModelIcon iconUrl={iconUrl} label={label} className="self-center" />
  );
}
