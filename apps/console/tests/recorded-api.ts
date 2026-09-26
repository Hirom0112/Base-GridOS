import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import type { Page } from "@playwright/test";

export async function recordedApi(page: Page) {
  await page.route("**/rpc/gridos.v1.*/*", async (route) => {
    const path = new URL(route.request().url()).pathname.replace(
      "/rpc/gridos.v1.",
      "",
    );
    const body = await readFile(
      resolve(process.cwd(), `../../testdata/fixtures/api/${path}.json`),
      "utf8",
    );
    await route.fulfill({ contentType: "application/json", body });
  });
}
