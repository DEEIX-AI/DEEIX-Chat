type QRVersionSpec = {
  version: number;
  dataCodewords: number;
  ecCodewordsPerBlock: number;
  blocks: number;
  alignment: number[];
};

const QR_VERSION_SPECS: QRVersionSpec[] = [
  { version: 5, dataCodewords: 108, ecCodewordsPerBlock: 26, blocks: 1, alignment: [6, 30] },
  { version: 6, dataCodewords: 136, ecCodewordsPerBlock: 18, blocks: 2, alignment: [6, 34] },
  { version: 7, dataCodewords: 156, ecCodewordsPerBlock: 20, blocks: 2, alignment: [6, 22, 38] },
  { version: 8, dataCodewords: 194, ecCodewordsPerBlock: 24, blocks: 2, alignment: [6, 24, 42] },
  { version: 9, dataCodewords: 232, ecCodewordsPerBlock: 30, blocks: 2, alignment: [6, 26, 46] },
];

type QRMatrix = {
  modules: boolean[][];
  reserved: boolean[][];
  size: number;
};

function escapeSVGAttribute(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("\"", "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

export function createQRCodeSVG(value: string, scale = 4, ariaLabel = "QR code"): string {
  const data = new TextEncoder().encode(value);
  const spec = QR_VERSION_SPECS.find((item) => data.length + 3 <= item.dataCodewords);
  if (!spec) {
    return "";
  }

  const codewords = createDataCodewords(data, spec.dataCodewords);
  const payload = interleaveBlocks(codewords, spec);
  const matrix = createMatrix(spec);
  placeData(matrix, payload);
  applyMask(matrix, 0);
  placeFormatBits(matrix, 0);

  const quiet = 4;
  const viewSize = matrix.size + quiet * 2;
  const rects: string[] = [];
  for (let y = 0; y < matrix.size; y += 1) {
    for (let x = 0; x < matrix.size; x += 1) {
      if (matrix.modules[y][x]) {
        rects.push(`<rect x="${x + quiet}" y="${y + quiet}" width="1" height="1"/>`);
      }
    }
  }

  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${viewSize} ${viewSize}" width="${viewSize * scale}" height="${viewSize * scale}" shape-rendering="crispEdges" role="img" aria-label="${escapeSVGAttribute(ariaLabel)}"><rect width="100%" height="100%" fill="#fff"/> <g fill="#000">${rects.join("")}</g></svg>`;
}

function createDataCodewords(data: Uint8Array, totalCodewords: number): number[] {
  const bits: number[] = [];
  appendBits(bits, 0b0100, 4);
  appendBits(bits, data.length, 8);
  data.forEach((byte) => appendBits(bits, byte, 8));
  appendBits(bits, 0, Math.min(4, totalCodewords * 8 - bits.length));
  while (bits.length % 8 !== 0) {
    bits.push(0);
  }

  const codewords: number[] = [];
  for (let index = 0; index < bits.length; index += 8) {
    let value = 0;
    for (let offset = 0; offset < 8; offset += 1) {
      value = (value << 1) | bits[index + offset];
    }
    codewords.push(value);
  }

  let pad = 0xec;
  while (codewords.length < totalCodewords) {
    codewords.push(pad);
    pad = pad === 0xec ? 0x11 : 0xec;
  }
  return codewords;
}

function appendBits(bits: number[], value: number, length: number) {
  for (let index = length - 1; index >= 0; index -= 1) {
    bits.push((value >>> index) & 1);
  }
}

function interleaveBlocks(codewords: number[], spec: QRVersionSpec): number[] {
  const dataBlocks = splitBlocks(codewords, spec.blocks);
  const ecBlocks = dataBlocks.map((block) => reedSolomonCompute(block, spec.ecCodewordsPerBlock));
  const result: number[] = [];

  const maxDataLength = Math.max(...dataBlocks.map((block) => block.length));
  for (let index = 0; index < maxDataLength; index += 1) {
    for (const block of dataBlocks) {
      if (index < block.length) {
        result.push(block[index]);
      }
    }
  }
  for (let index = 0; index < spec.ecCodewordsPerBlock; index += 1) {
    for (const block of ecBlocks) {
      result.push(block[index]);
    }
  }
  return result;
}

function splitBlocks(codewords: number[], blockCount: number): number[][] {
  const baseLength = Math.floor(codewords.length / blockCount);
  const remainder = codewords.length % blockCount;
  const blocks: number[][] = [];
  let offset = 0;
  for (let index = 0; index < blockCount; index += 1) {
    const length = baseLength + (index >= blockCount - remainder ? 1 : 0);
    blocks.push(codewords.slice(offset, offset + length));
    offset += length;
  }
  return blocks;
}

function reedSolomonCompute(data: number[], degree: number): number[] {
  const generator = reedSolomonGenerator(degree);
  const result = new Array<number>(degree).fill(0);
  for (const byte of data) {
    const factor = byte ^ result.shift()!;
    result.push(0);
    for (let index = 0; index < degree; index += 1) {
      result[index] ^= gfMultiply(generator[index], factor);
    }
  }
  return result;
}

function reedSolomonGenerator(degree: number): number[] {
  let result = [1];
  for (let index = 0; index < degree; index += 1) {
    const next = new Array<number>(result.length + 1).fill(0);
    for (let j = 0; j < result.length; j += 1) {
      next[j] ^= gfMultiply(result[j], 1);
      next[j + 1] ^= gfMultiply(result[j], gfPow(2, index));
    }
    result = next;
  }
  return result.slice(1);
}

function gfMultiply(a: number, b: number): number {
  let result = 0;
  let left = a;
  let right = b;
  while (right > 0) {
    if ((right & 1) !== 0) {
      result ^= left;
    }
    left <<= 1;
    if ((left & 0x100) !== 0) {
      left ^= 0x11d;
    }
    right >>>= 1;
  }
  return result;
}

function gfPow(value: number, power: number): number {
  let result = 1;
  for (let index = 0; index < power; index += 1) {
    result = gfMultiply(result, value);
  }
  return result;
}

function createMatrix(spec: QRVersionSpec): QRMatrix {
  const size = spec.version * 4 + 17;
  const matrix: QRMatrix = {
    modules: Array.from({ length: size }, () => new Array<boolean>(size).fill(false)),
    reserved: Array.from({ length: size }, () => new Array<boolean>(size).fill(false)),
    size,
  };

  placeFinder(matrix, 0, 0);
  placeFinder(matrix, size - 7, 0);
  placeFinder(matrix, 0, size - 7);
  placeTiming(matrix);
  placeAlignment(matrix, spec.alignment);
  reserveFormat(matrix);
  placeVersionInfo(matrix, spec.version);
  setModule(matrix, 8, spec.version * 4 + 9, true, true);
  return matrix;
}

function setModule(matrix: QRMatrix, x: number, y: number, value: boolean, reserved = false) {
  if (x < 0 || y < 0 || x >= matrix.size || y >= matrix.size) return;
  matrix.modules[y][x] = value;
  if (reserved) matrix.reserved[y][x] = true;
}

function placeFinder(matrix: QRMatrix, x: number, y: number) {
  for (let dy = -1; dy <= 7; dy += 1) {
    for (let dx = -1; dx <= 7; dx += 1) {
      const xx = x + dx;
      const yy = y + dy;
      if (xx < 0 || yy < 0 || xx >= matrix.size || yy >= matrix.size) continue;
      const inFinder = dx >= 0 && dx <= 6 && dy >= 0 && dy <= 6;
      const value = inFinder && (dx === 0 || dx === 6 || dy === 0 || dy === 6 || (dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4));
      setModule(matrix, xx, yy, value, true);
    }
  }
}

function placeTiming(matrix: QRMatrix) {
  for (let index = 8; index < matrix.size - 8; index += 1) {
    const value = index % 2 === 0;
    setModule(matrix, index, 6, value, true);
    setModule(matrix, 6, index, value, true);
  }
}

function placeAlignment(matrix: QRMatrix, positions: number[]) {
  for (const y of positions) {
    for (const x of positions) {
      if (matrix.reserved[y]?.[x]) continue;
      for (let dy = -2; dy <= 2; dy += 1) {
        for (let dx = -2; dx <= 2; dx += 1) {
          const value = Math.max(Math.abs(dx), Math.abs(dy)) !== 1;
          setModule(matrix, x + dx, y + dy, value, true);
        }
      }
    }
  }
}

function reserveFormat(matrix: QRMatrix) {
  for (let index = 0; index < 9; index += 1) {
    if (index !== 6) {
      matrix.reserved[8][index] = true;
      matrix.reserved[index][8] = true;
    }
  }
  for (let index = 0; index < 8; index += 1) {
    matrix.reserved[8][matrix.size - 1 - index] = true;
    matrix.reserved[matrix.size - 1 - index][8] = true;
  }
}

function placeVersionInfo(matrix: QRMatrix, version: number) {
  if (version < 7) return;
  const bits = calculateVersionBits(version);
  for (let index = 0; index < 18; index += 1) {
    const value = ((bits >> index) & 1) !== 0;
    setModule(matrix, matrix.size - 11 + (index % 3), Math.floor(index / 3), value, true);
    setModule(matrix, Math.floor(index / 3), matrix.size - 11 + (index % 3), value, true);
  }
}

function placeData(matrix: QRMatrix, codewords: number[]) {
  const bits = codewords.flatMap((byte) => Array.from({ length: 8 }, (_, index) => (byte >>> (7 - index)) & 1));
  let bitIndex = 0;
  let upward = true;
  for (let right = matrix.size - 1; right >= 1; right -= 2) {
    if (right === 6) right -= 1;
    for (let vertical = 0; vertical < matrix.size; vertical += 1) {
      const y = upward ? matrix.size - 1 - vertical : vertical;
      for (let column = 0; column < 2; column += 1) {
        const x = right - column;
        if (!matrix.reserved[y][x]) {
          setModule(matrix, x, y, bitIndex < bits.length && bits[bitIndex] === 1);
          bitIndex += 1;
        }
      }
    }
    upward = !upward;
  }
}

function applyMask(matrix: QRMatrix, mask: number) {
  for (let y = 0; y < matrix.size; y += 1) {
    for (let x = 0; x < matrix.size; x += 1) {
      if (!matrix.reserved[y][x] && shouldMask(mask, x, y)) {
        matrix.modules[y][x] = !matrix.modules[y][x];
      }
    }
  }
}

function shouldMask(mask: number, x: number, y: number) {
  switch (mask) {
    case 0:
      return (x + y) % 2 === 0;
    default:
      return false;
  }
}

function placeFormatBits(matrix: QRMatrix, mask: number) {
  const bits = calculateFormatBits(mask);
  for (let index = 0; index <= 5; index += 1) setModule(matrix, 8, index, ((bits >> index) & 1) !== 0, true);
  setModule(matrix, 8, 7, ((bits >> 6) & 1) !== 0, true);
  setModule(matrix, 8, 8, ((bits >> 7) & 1) !== 0, true);
  setModule(matrix, 7, 8, ((bits >> 8) & 1) !== 0, true);
  for (let index = 9; index < 15; index += 1) setModule(matrix, 14 - index, 8, ((bits >> index) & 1) !== 0, true);
  for (let index = 0; index < 8; index += 1) setModule(matrix, matrix.size - 1 - index, 8, ((bits >> index) & 1) !== 0, true);
  for (let index = 8; index < 15; index += 1) setModule(matrix, 8, matrix.size - 15 + index, ((bits >> index) & 1) !== 0, true);
}

function calculateFormatBits(mask: number): number {
  let data = (0b01 << 3) | mask;
  let value = data << 10;
  const generator = 0x537;
  for (let bit = 14; bit >= 10; bit -= 1) {
    if (((value >> bit) & 1) !== 0) value ^= generator << (bit - 10);
  }
  return ((data << 10) | value) ^ 0x5412;
}

function calculateVersionBits(version: number): number {
  let value = version << 12;
  const generator = 0x1f25;
  for (let bit = 17; bit >= 12; bit -= 1) {
    if (((value >> bit) & 1) !== 0) value ^= generator << (bit - 12);
  }
  return (version << 12) | value;
}
