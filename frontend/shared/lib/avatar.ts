import { pathParam, resolveApiBaseURL } from "@/shared/api/http-client";

type AvatarSeedSource = {
  publicID?: string | null;
  username?: string | null;
  displayName?: string | null;
};

const GENERATED_AVATAR_PREFIX = "generated:github:";
const FILE_AVATAR_PREFIX = "file:";

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

export function createFileAvatarRef(fileID: string) {
  return `${FILE_AVATAR_PREFIX}${fileID.trim()}`;
}

export function parseFileAvatarID(value: string) {
  if (!value.startsWith(FILE_AVATAR_PREFIX)) {
    return null;
  }

  const fileID = value.slice(FILE_AVATAR_PREFIX.length).trim();
  return fileID.startsWith("file_") ? fileID : null;
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
  const canvasSize = 96;
  const gridSize = 5;
  const cellSize = 15;
  const padding = (canvasSize - gridSize * cellSize) / 2;
  const hue = state % 360;
  const backgroundColor = `hsl(${(hue + 8) % 360} ${18 + (state % 8)}% ${88 + (state % 5)}%)`;
  const foregroundColor = `hsl(${hue} ${42 + (state % 10)}% ${28 + (state % 8)}%)`;
  const cells: string[] = [];
  let grid = Array.from({ length: gridSize }, () => Array.from({ length: gridSize }, () => false));

  const nextValue = () => {
    state ^= state << 13;
    state ^= state >>> 17;
    state ^= state << 5;
    return state >>> 0;
  };

  const setMirroredCell = (row: number, column: number, filled: boolean) => {
    grid[row][column] = filled;
    grid[row][gridSize - 1 - column] = filled;
  };

  const mirroredCellWeight = (column: number) => (column === Math.floor(gridSize / 2) ? 1 : 2);

  for (let row = 0; row < gridSize; row += 1) {
    for (let column = 0; column < Math.ceil(gridSize / 2); column += 1) {
      const columnBias = [34, 30, 28][column] ?? 30;
      const rowBias = [0, 2, 4, 2, 0][row] ?? 0;
      setMirroredCell(row, column, nextValue() % 100 < columnBias + rowBias);
    }
  }

  const countFilledCells = () => {
    let count = 0;
    for (const rowCells of grid) {
      for (const filled of rowCells) {
        if (filled) {
          count += 1;
        }
      }
    }
    return count;
  };

  const shuffledMirroredCells = Array.from({ length: gridSize * Math.ceil(gridSize / 2) }, (_, index) => [
    Math.floor(index / Math.ceil(gridSize / 2)),
    index % Math.ceil(gridSize / 2),
  ] as const);

  for (let index = shuffledMirroredCells.length - 1; index > 0; index -= 1) {
    const swapIndex = nextValue() % (index + 1);
    const current = shuffledMirroredCells[index];
    shuffledMirroredCells[index] = shuffledMirroredCells[swapIndex];
    shuffledMirroredCells[swapIndex] = current;
  }

  for (const [row, column] of shuffledMirroredCells) {
    if (countFilledCells() >= 8) {
      break;
    }
    if (!grid[row][column]) {
      setMirroredCell(row, column, true);
    }
  }

  for (const [row, column] of shuffledMirroredCells) {
    if (countFilledCells() <= 11) {
      break;
    }
    if (grid[row][column] && countFilledCells() - mirroredCellWeight(column) >= 8) {
      setMirroredCell(row, column, false);
    }
  }

  for (let row = 0; row < gridSize; row += 1) {
    for (let column = 0; column < gridSize; column += 1) {
      if (!grid[row][column]) {
        continue;
      }

      const x = padding + column * cellSize;
      const y = padding + row * cellSize;
      cells.push(`<rect x="${x}" y="${y}" width="${cellSize}" height="${cellSize}" fill="${foregroundColor}" />`);
    }
  }

  const svg = [
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${canvasSize} ${canvasSize}" fill="none">`,
    `<rect width="${canvasSize}" height="${canvasSize}" rx="12" fill="${backgroundColor}" />`,
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

  const fileID = parseFileAvatarID(normalizedAvatarURL);
  const publicID = normalizeString(source?.publicID);
  if (fileID && publicID) {
    return `${resolveApiBaseURL()}/api/v1/users/${pathParam(publicID)}/avatar?file=${pathParam(fileID)}`;
  }

  return normalizedAvatarURL;
}
