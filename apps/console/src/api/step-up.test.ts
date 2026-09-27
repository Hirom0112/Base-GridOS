import { expect, test, vi } from "vitest";
import { createConsoleClient } from "./client";

function response(body: object, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

for (const action of ["approve", "stop"] as const) {
  test(`${action} obtains a bound assertion before sending the command`, async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(response({ assertion: "signed-assertion" }))
      .mockResolvedValueOnce(response({}));
    const client = createConsoleClient(
      "http://localhost/rpc",
      {
        mode: "local",
        role: "approver",
        permissions: [],
      },
      fetcher,
    );
    if (action === "approve")
      await client.dispatch.approveEvent({
        eventId: "event-1",
        planVersion: 4n,
        idempotencyKey: "intent-1",
      });
    else
      await client.events.emergencyStop({
        eventId: "event-1",
        idempotencyKey: "intent-1",
      });
    expect(fetcher).toHaveBeenCalledTimes(2);
    const [url, init] = fetcher.mock.calls[0]!;
    expect(url).toBe("/local/step-up");
    expect(init?.method).toBe("POST");
    expect(init?.signal).toBeInstanceOf(AbortSignal);
    expect(new Headers(init?.headers).get("X-GridOS-Role")).toBe("approver");
    expect(JSON.parse(String(init?.body))).toEqual({
      action: action === "approve" ? "APPROVE_EVENT" : "EMERGENCY_STOP",
      event_id: "event-1",
      plan_version: action === "approve" ? 4 : 0,
    });
    const command = fetcher.mock.calls[1]!;
    expect(new Headers(command[1]?.headers).get("X-GridOS-Step-Up")).toBe(
      "signed-assertion",
    );
    expect(await new Response(command[1]?.body).json()).toMatchObject({
      idempotencyKey: "intent-1",
    });
  });

  test.each([403, 503, 200])(
    `${action} sends no command when assertion acquisition fails (%i)`,
    async (status) => {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockResolvedValue(response({}, status));
      const client = createConsoleClient(
        "http://localhost/rpc",
        {
          mode: "local",
          role: "approver",
          permissions: [],
        },
        fetcher,
      );
      const command =
        action === "approve"
          ? client.dispatch.approveEvent({
              eventId: "event-1",
              planVersion: 4n,
            })
          : client.events.emergencyStop({ eventId: "event-1" });
      await expect(command).rejects.toThrow(/authorization|assertion/i);
      expect(fetcher).toHaveBeenCalledTimes(1);
      expect(fetcher.mock.calls[0]?.[0]).toBe("/local/step-up");
    },
  );
}

test("command retries acquire fresh assertions and retain the same intent", async () => {
  const fetcher = vi
    .fn<typeof fetch>()
    .mockResolvedValueOnce(response({ assertion: "first" }))
    .mockRejectedValueOnce(new TypeError("Connection lost"))
    .mockResolvedValueOnce(response({ assertion: "second" }))
    .mockResolvedValueOnce(response({}));
  const client = createConsoleClient(
    "http://localhost/rpc",
    {
      mode: "local",
      role: "approver",
      permissions: [],
    },
    fetcher,
  );
  const intent = {
    eventId: "event-1",
    planVersion: 4n,
    idempotencyKey: "stable-intent",
  };
  await expect(client.dispatch.approveEvent(intent)).rejects.toThrow(
    "Connection lost",
  );
  await client.dispatch.approveEvent(intent);
  expect(fetcher.mock.calls[1]?.[1]?.body).toEqual(
    fetcher.mock.calls[3]?.[1]?.body,
  );
  expect(
    new Headers(fetcher.mock.calls[3]?.[1]?.headers).get("X-GridOS-Step-Up"),
  ).toBe("second");
});

test("launch uses its ordinary authorization without an approval assertion", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(response({}));
  const client = createConsoleClient(
    "http://localhost/rpc",
    {
      mode: "local",
      role: "approver",
      permissions: [],
    },
    fetcher,
  );
  await client.dispatch.launchEvent({ eventId: "event-1", planVersion: 4n });
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(fetcher.mock.calls[0]?.[0]).toContain("/LaunchEvent");
  expect(
    new Headers(fetcher.mock.calls[0]?.[1]?.headers).has("X-GridOS-Step-Up"),
  ).toBe(false);
});

test("production identities never fall back to the local signer", async () => {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(response({}));
  const client = createConsoleClient(
    "http://localhost/rpc",
    {
      mode: "clerk",
      role: "approver",
      userId: "user-1",
      getToken: async () => "session-token",
    },
    fetcher,
  );
  await expect(
    client.dispatch.approveEvent({ eventId: "event-1", planVersion: 1n }),
  ).rejects.toThrow(/step-up/i);
  expect(fetcher).not.toHaveBeenCalled();
});
