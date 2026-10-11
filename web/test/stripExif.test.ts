import { describe, expect, it } from "vitest";
import { stripLocation } from "../lib/stripExif";

const enc = new TextEncoder();
const has = (b: Uint8Array, s: string) => Buffer.from(b).includes(Buffer.from(s));

function segment(marker: number, payload: string): number[] {
  const p = Array.from(enc.encode(payload));
  const len = p.length + 2;
  return [0xff, marker, len >> 8, len & 0xff, ...p];
}

function jpeg(...segs: number[][]): Uint8Array {
  // SOI, segments, a fake SOS with scan data, EOI
  return new Uint8Array([0xff, 0xd8, ...segs.flat(), 0xff, 0xda, 0x00, 0x02, 0x11, 0x22, 0x33, 0xff, 0xd9]);
}

describe("stripLocation (FR-047)", () => {
  it("removes EXIF (GPS, serial) from a JPEG and keeps XMP provenance", () => {
    const input = jpeg(
      segment(0xe1, "Exif\0\0GPSLatitude 51.5362 SerialNumber 9"),
      segment(0xe1, "http://ns.adobe.com/xap/1.0/\0<x DigitalSourceType='trainedAlgorithmicMedia'/>"),
      segment(0xeb, "jumb c2pa manifest"),
    );
    const out = stripLocation(input);
    expect(has(out, "GPSLatitude")).toBe(false);
    expect(has(out, "SerialNumber")).toBe(false);
    expect(has(out, "trainedAlgorithmicMedia")).toBe(true);
    expect(has(out, "c2pa")).toBe(true);
    expect(Array.from(out.slice(-5))).toEqual([0x11, 0x22, 0x33, 0xff, 0xd9]); // image data intact
  });

  it("removes location chunks from a PNG and keeps the image", () => {
    const chunk = (type: string, data: string) => {
      const d = Array.from(enc.encode(data));
      const len = d.length;
      return [len >>> 24, (len >> 16) & 255, (len >> 8) & 255, len & 255, ...Array.from(enc.encode(type)), ...d, 0, 0, 0, 0];
    };
    const png = new Uint8Array([
      0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
      ...chunk("IHDR", "hdr"), ...chunk("eXIf", "GPSLatitude"), ...chunk("tEXt", "Location: home"),
      ...chunk("iTXt", "XML:com.adobe.xmp provenance"), ...chunk("IDAT", "pixels"), ...chunk("IEND", ""),
    ]);
    const out = stripLocation(png);
    expect(has(out, "GPSLatitude")).toBe(false);
    expect(has(out, "Location: home")).toBe(false);
    expect(has(out, "provenance")).toBe(true);
    expect(has(out, "pixels")).toBe(true);
  });

  it("leaves other files untouched", () => {
    const other = enc.encode("GIF89a...");
    expect(stripLocation(other)).toEqual(other);
  });
});
