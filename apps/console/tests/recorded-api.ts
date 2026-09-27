import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import type { Page } from "@playwright/test";

export async function recordedApi(page: Page) {
  await page.route("**/rpc/gridos.v1.*/*", async (route) => {
    const path = new URL(route.request().url()).pathname.replace(
      "/rpc/gridos.v1.",
      "",
    );
    if (path === "EventsService/WatchEvent") {
      await route.fulfill({
        status: 501,
        contentType: "application/json",
        body: JSON.stringify({
          code: "unimplemented",
          message: "This unary fixture set contains no event stream",
        }),
      });
      return;
    }
    const body = await readFile(
      resolve(process.cwd(), `../../testdata/fixtures/api/${path}.json`),
      "utf8",
    ).catch((error: unknown) => {
      if (
        [
          "EventsService/GetEventTimeline",
          "EventsService/ListEventCommands",
          "MemberService/ListMemberOffers",
        ].includes(path) &&
        error instanceof Error &&
        "code" in error &&
        error.code === "ENOENT"
      )
        return null;
      throw error;
    });
    if (body === null) {
      await route.fulfill({
        status: 501,
        contentType: "application/json",
        body: JSON.stringify({
          code: "unimplemented",
          message: "Recording not available for this evidence",
        }),
      });
      return;
    }
    await route.fulfill({ contentType: "application/json", body });
  });
}
