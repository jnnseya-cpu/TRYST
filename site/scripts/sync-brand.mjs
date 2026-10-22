// Copies the logo (byte-for-byte, checksum-verified, 01 §3.1) and colour tokens from
// ../shared/brand into the site.
import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const brand = join(here, "..", "..", "shared", "brand");
const LOGO_SHA256 = "5f2ceb55817d6cc653da5e5c133edf30b7c809ce326ff52afcf98471eeae2761";
mkdirSync(join(here, "..", "public", "brand"), { recursive: true });
copyFileSync(join(brand, "tryst-logo.png"), join(here, "..", "public", "brand", "tryst-logo.png"));
copyFileSync(join(brand, "tokens.css"), join(here, "..", "app", "brand-tokens.css"));
const got = createHash("sha256").update(readFileSync(join(here, "..", "public", "brand", "tryst-logo.png"))).digest("hex");
if (got !== LOGO_SHA256) {
  console.error(`Logo checksum mismatch: ${got}. The logo must be used exactly as supplied.`);
  process.exit(1);
}
console.log("brand assets synced; logo checksum verified");
