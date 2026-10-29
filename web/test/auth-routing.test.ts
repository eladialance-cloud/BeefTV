import { describe, expect, test } from "bun:test";

import { loginRedirectPath, shouldRedirectToLogin } from "../src/lib/auth-routing";

describe("hosted session replacement redirect", () => {
    test("protected 401 responses return the user to login", () => {
        expect(shouldRedirectToLogin(401, "/canvas/c1")).toBe(true);
        expect(shouldRedirectToLogin(401, "/login")).toBe(false);
        expect(shouldRedirectToLogin(403, "/canvas/c1")).toBe(false);
    });

    test("redirect keeps the current app path and marks the session replacement", () => {
        expect(loginRedirectPath("/canvas/c1", "?node=n1")).toBe(
            "/login?next=%2Fcanvas%2Fc1%3Fnode%3Dn1&reason=session_replaced",
        );
    });
});
