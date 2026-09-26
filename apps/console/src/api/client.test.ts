import { create, toJsonString } from "@bufbuild/protobuf";
import { expect, test, vi } from "vitest";
import { createConsoleClient } from "./client";
import { GetFleetSummaryResponseSchema } from "./gen/gridos/v1/api_pb";

test("client sends the selected local identity and a bounded timeout", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockResolvedValue(
      new Response(
        toJsonString(
          GetFleetSummaryResponseSchema,
          create(GetFleetSummaryResponseSchema),
        ),
        { headers: { "content-type": "application/json" } },
      ),
    );
  const client = createConsoleClient(
    "http://localhost/rpc",
    { mode: "local", role: "analyst", permissions: [] },
    fetcher,
  );
  await client.fleet.getFleetSummary({});
  const request = fetcher.mock.calls[0]?.[0];
  expect(request).toBeInstanceOf(Request);
  if (!(request instanceof Request)) throw new Error("Expected a request");
  expect(request.headers.get("X-GridOS-Role")).toBe("analyst");
  expect(request.headers.get("Connect-Timeout-Ms")).toBeTruthy();
  expect(request.headers.has("X-GridOS-Permissions")).toBe(false);
});

test("client preserves permission failures and never retries mutations", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
    new Response(
      JSON.stringify({
        code: "permission_denied",
        message: "approver required",
      }),
      { status: 403, headers: { "content-type": "application/json" } },
    ),
  );
  const client = createConsoleClient(
    "http://localhost/rpc",
    { mode: "local", role: "operator", permissions: [] },
    fetcher,
  );
  await expect(
    client.dispatch.approveEvent({
      eventId: "event",
      planVersion: 1n,
      idempotencyKey: "stable-command",
    }),
  ).rejects.toMatchObject({ code: 7 });
  expect(fetcher).toHaveBeenCalledTimes(1);
});
