import { ModelIcon } from "@/shared/components/model-icon";

export function ModelOptionIcon({
  iconUrl,
  label,
  size = 16,
}: {
  iconUrl?: string | null;
  label: string;
  size?: number;
}) {
  return (
    <ModelIcon iconUrl={iconUrl} label={label} size={size} className="self-center" />
  );
}
