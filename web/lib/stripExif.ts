// Remove location and device data from a photo on the device before upload (FR-047, FR-093).
//
// JPEG: drops APP1 "Exif" segments (GPS position, camera serial, owner name, timestamps).
// PNG:  drops eXIf chunks and textual chunks that cameras and editors use for the same data.
// Keeps XMP (APP1 "http://ns.adobe.com/xap/1.0/") and C2PA (APP11 / caBX) so the server can
// read the provenance labels (generator markers); the server then re-encodes and strips everything.

const PNG_SIG = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
const EXIF_ID = [0x45, 0x78, 0x69, 0x66, 0x00, 0x00]; // "Exif\0\0"

function startsWith(b: Uint8Array, at: number, sig: number[]): boolean {
  if (at + sig.length > b.length) return false;
  return sig.every((v, i) => b[at + i] === v);
}

export function stripLocation(input: Uint8Array): Uint8Array {
  if (input.length > 3 && input[0] === 0xff && input[1] === 0xd8) return stripJpeg(input);
  if (startsWith(input, 0, PNG_SIG)) return stripPng(input);
  return input;
}

function stripJpeg(b: Uint8Array): Uint8Array {
  const out: number[] = [0xff, 0xd8];
  let i = 2;
  while (i + 4 <= b.length) {
    if (b[i] !== 0xff) break; // malformed: keep the rest as is
    const marker = b[i + 1];
    if (marker === 0xda || marker === 0xd9) break; // start of scan / end: copy the rest
    const len = (b[i + 2] << 8) | b[i + 3];
    if (len < 2 || i + 2 + len > b.length) break;
    const isExif = marker === 0xe1 && startsWith(b, i + 4, EXIF_ID);
    if (!isExif) for (let k = i; k < i + 2 + len; k++) out.push(b[k]);
    i += 2 + len;
  }
  const rest = b.subarray(i);
  const result = new Uint8Array(out.length + rest.length);
  result.set(out, 0);
  result.set(rest, out.length);
  return result;
}

const DROP_PNG = new Set(["eXIf", "tEXt", "zTXt", "tIME"]);

function stripPng(b: Uint8Array): Uint8Array {
  const parts: Uint8Array[] = [b.subarray(0, 8)];
  let i = 8;
  while (i + 12 <= b.length) {
    const len = ((b[i] << 24) >>> 0) + (b[i + 1] << 16) + (b[i + 2] << 8) + b[i + 3];
    const type = String.fromCharCode(b[i + 4], b[i + 5], b[i + 6], b[i + 7]);
    const end = i + 12 + len;
    if (end > b.length) {
      parts.push(b.subarray(i));
      i = b.length;
      break;
    }
    // iTXt may hold XMP (provenance): keep it. Location-bearing chunks are dropped.
    if (!DROP_PNG.has(type)) parts.push(b.subarray(i, end));
    i = end;
  }
  if (i < b.length) parts.push(b.subarray(i));
  const total = parts.reduce((n, p) => n + p.length, 0);
  const out = new Uint8Array(total);
  let o = 0;
  for (const p of parts) {
    out.set(p, o);
    o += p.length;
  }
  return out;
}
