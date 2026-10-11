// On-device message-safety baseline (FR-102, FR-110): the browser port of
// backend/internal/guardian. Same patterns, same labelled vectors
// (contracts/vocab/guardian_vectors.json). Runs on the member's device only; the server
// receives the signal kind, never the message.

export type Signal = { kind: "child_safety" | "extortion" | "fraud"; action: "freeze_urgent" | "freeze_support" | "warn_recipient" };

const re = (s: string) => new RegExp(s, "i");

const money = String.raw`(?:money|cash|£\s?\d+|\$\s?\d+|\d+\s?(?:quid|pounds|gbp)|funds|rent)`;
const fraud = [
  re(String.raw`\b(?:send|transfer|wire|lend|loan|give)\s+(?:me\s+)?(?:some\s+)?` + money),
  re(String.raw`\b(?:transfer|send|wire)\s+(?:me\s+)?(?:the\s+)?money\b`),
  re(String.raw`\b(?:gift|steam|itunes|apple|amazon|google\s*play)\s*cards?\b`),
  re(String.raw`\b(?:crypto|bitcoin|btc|usdt|ethereum)\b.{0,60}\b(?:invest|platform|wallet|returns?|profit|trading)\b`),
  re(String.raw`\b(?:invest(?:ment)?|trading)\s+(?:platform|opportunity|app)\b`),
  re(String.raw`\bguaranteed\s+(?:returns?|profit)\b`),
  re(String.raw`\bwallet\s+address\b`),
  re(String.raw`\b(?:western\s+union|moneygram)\b`),
  re(String.raw`\bpay\s+(?:for\s+)?my\s+(?:ticket|flight|taxi|visa|hospital|bills?|fine)\b`),
];

const child = String.raw`(?:kids?|child(?:ren)?|son|daughter|little\s+(?:girl|boy)|step-?(?:son|daughter))`;
const childSafety = [
  re(String.raw`\b(?:pics?|photos?|pictures?|videos?)\s+of\s+(?:your\s+)?` + child + String.raw`\b`),
  re(String.raw`\b(?:your\s+)?` + child + String.raw`\b.{0,25}\b(?:home\s+)?alone\b`),
  re(String.raw`\bbring\s+(?:your\s+)?` + child + String.raw`\b`),
  re(String.raw`\bbabysit\b.{0,40}\b(?:just\s+me|alone|them)\b`),
  re(String.raw`\bpretend\s+(?:you'?re|to\s+be)\s+(?:1[0-7]|[1-9])\b`),
  re(String.raw`\bhow\s+old\s+is\s+your\s+` + child + String.raw`\b.{0,40}\b(?:pretty|cute|hot|sexy|developed)\b`),
  re(String.raw`\b(?:like|want|into)\s+(?:them\s+)?(?:young|little)\b.{0,20}\b(?:1[0-7]|[1-9])\b`),
  re(String.raw`\b(?:underage|preteen|pre-teen|jailbait)\b`),
];

const who = String.raw`(?:wife|husband|partner|boss|employer|work|family|kids|friends|everyone|church|mum|dad)`;
const extortion = [
  re(String.raw`\b(?:pay|send|transfer)\b.{0,40}\b(?:or|unless|otherwise)\b.{0,40}\b(?:tell|show|send|go\s+to|post)\b.{0,30}\byour\s+` + who),
  re(String.raw`\b(?:tell|show|send)\b.{0,15}\byour\s+` + who + String.raw`\b.{0,60}\b(?:unless|if\s+you\s+don'?t|or\s+else)\b`),
  re(String.raw`\b(?:unless|if\s+you\s+don'?t|or\s+(?:i'?ll|i\s+will))\b.{0,60}\b(?:tell|show|send)\b.{0,15}\byour\s+` + who),
  re(String.raw`\b(?:pay|send|money|£|cash)\b.{0,40}\bunless\b.{0,30}\byour\s+` + who + String.raw`\b.{0,30}\b(?:see|know|find\s+out|hear)\b`),
  re(String.raw`\bexpose\s+you\b`),
  re(String.raw`\beveryone\s+will\s+know\b`),
  re(String.raw`\b(?:pics|photos|screenshots|videos)\b.{0,30}\b(?:go|going|sent)\s+to\s+your\s+` + who),
  re(String.raw`\bi\s+know\s+where\s+you\s+(?:work|live)\b`),
];

function normalise(s: string): string {
  return s.toLowerCase().replace(/[’‘]/g, "'").replace(/[​‍]/g, "").split(/\s+/).filter(Boolean).join(" ");
}

export function check(message: string): Signal[] {
  const s = normalise(message);
  const out: Signal[] = [];
  if (childSafety.some((p) => p.test(s))) out.push({ kind: "child_safety", action: "freeze_urgent" });
  if (extortion.some((p) => p.test(s))) out.push({ kind: "extortion", action: "freeze_support" });
  if (fraud.some((p) => p.test(s))) out.push({ kind: "fraud", action: "warn_recipient" });
  return out;
}

export const MODEL = "baseline-v1";
