import { expect, test } from "bun:test";

import { DEFAULT_PUBLIC_APPEARANCE, normalizePublicAppearance } from "../src/stores/use-appearance-store";

test("public appearance falls back to the VisiDeep AI brand", () => {
    const appearance = normalizePublicAppearance();

    expect(DEFAULT_PUBLIC_APPEARANCE.brandName).toBe("VisiDeep AI");
    expect(appearance.brandName).toBe("VisiDeep AI");
    expect(appearance.brandSlug).toBe("visideep-ai");
    expect(appearance.logoUrl).toBe("/visideep-ai.png");
    expect(appearance.seoTitle).toBe("VisiDeep AI");
});
