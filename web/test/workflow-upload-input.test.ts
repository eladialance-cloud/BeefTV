import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

test("workflow photo fields use a native label-bound file input", () => {
    const source = readFileSync(resolve(import.meta.dir, "../src/components/canvas/canvas-workflow-setup-modal.tsx"), "utf8");

    expect(source).toContain('htmlFor={inputId}');
    expect(source).toContain('id={inputId}');
    expect(source).toContain('type="file"');
    expect(source).toContain('className="sr-only"');
    expect(source).toContain('role="alert"');
    expect(source).toContain("setUploadError");
    expect(source).toContain("readImageMeta");
    expect(source).toContain("width: imageMeta.width");
    expect(source).toContain("height: imageMeta.height");
    expect(source).toContain("forceRemote: true");
    expect(source).not.toContain("<Upload");
});
