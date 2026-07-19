import assert from "node:assert/strict";
import test from "node:test";

import { formatExtensionImportResult } from "../src/pages/console/manager/extension-package.ts";

test("格式化扩展包导入结果", () => {
  let translatedKey = ""
  let translatedOptions: Record<string, unknown> | undefined
  assert.equal(
    formatExtensionImportResult({
      created_skills: 1,
      updated_skills: 2,
      created_images: 3,
      updated_images: 4,
    }, (key, options) => {
      translatedKey = key
      translatedOptions = options
      return "translated summary"
    }),
    "translated summary",
  );
  assert.equal(translatedKey, "managerSkills.extensionImport.summary")
  assert.deepEqual(translatedOptions, {
    createdSkills: 1,
    updatedSkills: 2,
    createdImages: 3,
    updatedImages: 4,
  })
});

test("格式化扩展包导入结果时缺省计数按 0 处理", () => {
  assert.equal(
    formatExtensionImportResult({}, () => "translated summary"),
    "translated summary",
  );
});
