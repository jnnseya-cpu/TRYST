import { describe, expect, it } from "vitest";
import { generateDeviceKeys, open, publicJwk, seal } from "../lib/e2ee";

describe("thread encryption (FR-114)", () => {
  it("each device opens only its own envelope; nothing is readable in transit", async () => {
    const ana = await generateDeviceKeys();
    const ben = await generateDeviceKeys();
    const eve = await generateDeviceKeys();
    const envs = await seal("th_1", "m1", "See you at 8?", [
      { id: "ana-1", jwk: await publicJwk(ana) },
      { id: "ben-1", jwk: await publicJwk(ben) },
    ]);
    expect(envs).toHaveLength(2);
    for (const e of envs) expect(atob(e.ct)).not.toContain("See you");
    const forBen = envs.find((e) => e.device_id === "ben-1")!;
    expect(await open("th_1", "m1", forBen, ben)).toBe("See you at 8?");
    expect(await open("th_1", "m1", envs.find((e) => e.device_id === "ana-1")!, ana)).toBe("See you at 8?");
    await expect(open("th_1", "m1", forBen, eve)).rejects.toThrow(); // wrong device
    await expect(open("th_2", "m1", forBen, ben)).rejects.toThrow(); // moved to another thread
    await expect(open("th_1", "m2", forBen, ben)).rejects.toThrow(); // replayed as another message
    const tampered = { ...forBen, ct: forBen.ct.slice(0, -4) + (forBen.ct.endsWith("AAAA") ? "BBBB" : "AAAA") };
    await expect(open("th_1", "m1", tampered, ben)).rejects.toThrow();
  });

  it("the private key can't be exported", async () => {
    const k = await generateDeviceKeys();
    await expect(crypto.subtle.exportKey("jwk", k.privateKey)).rejects.toThrow();
    const pub = await publicJwk(k);
    expect(pub).not.toHaveProperty("d");
  });
});
