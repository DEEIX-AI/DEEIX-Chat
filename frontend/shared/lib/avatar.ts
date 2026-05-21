type AvatarSeedSource = {
  publicID?: string | null;
  username?: string | null;
  displayName?: string | null;
};

const GENERATED_AVATAR_PREFIX = "generated:github:";

function normalizeString(value: unknown, fallback = "") {
  if (typeof value !== "string") {
    return fallback;
  }

  const normalizedValue = value.trim();
  return normalizedValue || fallback;
}

function hashString(input: string) {
  let hash = 2166136261;

  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }

  return hash >>> 0;
}

export function createGeneratedGithubAvatarRef(variant: number) {
  return `${GENERATED_AVATAR_PREFIX}${Math.max(0, Math.trunc(variant))}`;
}

export function isGeneratedGithubAvatarRef(value: string) {
  return value.startsWith(GENERATED_AVATAR_PREFIX);
}

export function parseGeneratedGithubAvatarVariant(value: string) {
  if (!isGeneratedGithubAvatarRef(value)) {
    return null;
  }

  const parsedValue = Number.parseInt(value.slice(GENERATED_AVATAR_PREFIX.length), 10);
  if (!Number.isFinite(parsedValue) || parsedValue < 0) {
    return null;
  }

  return parsedValue;
}

export function generateAvatarVariant() {
  if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
    const values = new Uint32Array(1);
    crypto.getRandomValues(values);
    return values[0] ?? Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
  }

  return Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
}

export function resolveAvatarSeed(source?: AvatarSeedSource) {
  return (
    normalizeString(source?.publicID) ||
    normalizeString(source?.username) ||
    normalizeString(source?.displayName) ||
    "deeix-chat-user"
  );
}

export function createGithubStyleAvatar(seed: string, variant: number) {
  let state = hashString(`${seed}:${variant}`) || 1;
  const cellSize = 11;
  const padding = 7;
  const gridSize = 5;
  const canvasSize = padding * 2 + cellSize * gridSize;
  const hue = state % 360;
  const foregroundSaturation = 42 + (state % 10);
  const foregroundLightness = 28 + (state % 8);
  const backgroundHue = (hue + 8 + (state % 24)) % 360;
  const backgroundSaturation = 20 + (state % 10);
  const backgroundLightness = 84 + (state % 5);
  const foregroundColor = `hsl(${hue} ${foregroundSaturation}% ${foregroundLightness}%)`;
  const backgroundColor = `hsl(${backgroundHue} ${backgroundSaturation}% ${backgroundLightness}%)`;
  const cells: string[] = [];

  const nextValue = () => {
    state ^= state << 13;
    state ^= state >>> 17;
    state ^= state << 5;
    return state >>> 0;
  };

  for (let row = 0; row < gridSize; row += 1) {
    for (let column = 0; column < Math.ceil(gridSize / 2); column += 1) {
      const filled = nextValue() % 100 < 58;
      if (!filled) {
        continue;
      }

      const mirroredColumn = gridSize - 1 - column;
      const x = padding + column * cellSize;
      const y = padding + row * cellSize;
      cells.push(`<rect x="${x}" y="${y}" width="${cellSize - 1}" height="${cellSize - 1}" fill="${foregroundColor}" />`);

      if (mirroredColumn !== column) {
        const mirroredX = padding + mirroredColumn * cellSize;
        cells.push(`<rect x="${mirroredX}" y="${y}" width="${cellSize - 1}" height="${cellSize - 1}" fill="${foregroundColor}" />`);
      }
    }
  }

  const svg = [
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${canvasSize} ${canvasSize}" fill="none">`,
    `<rect width="${canvasSize}" height="${canvasSize}" rx="10" fill="${backgroundColor}" />`,
    ...cells,
    "</svg>",
  ].join("");

  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

export function resolveAvatarImageSrc(avatarURL: unknown, source?: AvatarSeedSource) {
  const normalizedAvatarURL = normalizeString(avatarURL);
  const generatedVariant = parseGeneratedGithubAvatarVariant(normalizedAvatarURL);
  if (generatedVariant !== null) {
    return createGithubStyleAvatar(resolveAvatarSeed(source), generatedVariant);
  }

  return normalizedAvatarURL;
}
