import { describe, expect, test } from "vitest";

import { formatCreditAmount } from "../src/services/api/credits";

describe("formatCreditAmount", () => {
    test("renders fixed-point wallet values as decimal credits", () => {
        expect(formatCreditAmount(1_000_000)).toBe("1");
        expect(formatCreditAmount(9_500_000)).toBe("9.5");
        expect(formatCreditAmount(250_000)).toBe("0.25");
        expect(formatCreditAmount(0)).toBe("0");
    });
});
