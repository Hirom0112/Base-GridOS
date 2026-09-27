import { waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { startErrorReporting } from "./error-reporting";

afterEach(() => vi.unstubAllGlobals());

test("the Sentry transport receives only the redacted event and detaches on exit", async () => {
  const fetcher = vi.fn().mockResolvedValue(new Response("{}"));
  vi.stubGlobal("fetch", fetcher);
  const stop = startErrorReporting(
    "https://public@errors.example.test/1",
    "report",
  );
  window.dispatchEvent(
    new ErrorEvent("error", {
      error: new Error("household-secret travel-active"),
      message: "site-secret",
      filename: "https://console.test/member/household-secret",
    }),
  );
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));
  const options: RequestInit = fetcher.mock.calls[0]![1];
  expect(String(options.body)).toContain("Console runtime error");
  expect(String(options.body)).toContain('"stage":"report"');
  expect(String(options.body)).not.toMatch(
    /household-secret|site-secret|travel-active|console.test/,
  );
  expect(options.credentials).toBe("omit");
  expect(options.referrerPolicy).toBe("no-referrer");
  stop();
  window.dispatchEvent(
    new ErrorEvent("error", { message: "member-view-error" }),
  );
  await new Promise((resolve) => setTimeout(resolve, 10));
  expect(fetcher).toHaveBeenCalledTimes(1);
});
