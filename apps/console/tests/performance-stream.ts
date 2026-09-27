import { readFile } from "node:fs/promises";
import { createServer, type ServerResponse } from "node:http";
import { create, fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import type { Page } from "@playwright/test";
import {
  GetEventResponseSchema,
  ListSitesResponseSchema,
  WatchEventResponseSchema,
} from "../src/api/gen/gridos/v1/api_pb";
import { DispatchEventState } from "../src/api/gen/gridos/v1/dispatch_pb";
import { ValueState } from "../src/api/gen/gridos/v1/telemetry_pb";

export async function performanceStream(page: Page) {
  const sites = fromJsonString(
    ListSitesResponseSchema,
    await readFile(
      "../../testdata/fixtures/api/FleetService/ListSites.json",
      "utf8",
    ),
  );
  const { event } = fromJsonString(
    GetEventResponseSchema,
    await readFile(
      "../../testdata/fixtures/api/DispatchService/GetEvent.json",
      "utf8",
    ),
  );
  if (!event) throw new Error("Recorded event missing");
  const cells = sites.sites.flatMap((site) =>
    site.location.case === "aggregate" ? [site.location.value] : [],
  );
  const clients = new Set<ServerResponse>();
  let revision = 0;
  function publish() {
    const observedAt = timestampFromDate(
      new Date(Date.UTC(2026, 8, 27, 12, 0, ++revision)),
    );
    const metadata = {
      timestamp: observedAt,
      freshness: {},
      provenanceMix: [{ provenance: 5, recordCount: BigInt(cells.length) }],
    };
    const power = {
      sentMw: 0.01,
      acknowledgedMw: 0.008,
      deliveredMw: 0.006 + revision / 10000,
      deliveredState: ValueState.PRESENT,
      metadata,
    };
    const body = Buffer.from(
      toJsonString(
        WatchEventResponseSchema,
        create(WatchEventResponseSchema, {
          event: {
            eventId: event!.eventId,
            state: DispatchEventState.EXECUTING,
          },
          observedAt,
          fleet: {
            ...power,
            sentMw: power.sentMw * cells.length,
            acknowledgedMw: power.acknowledgedMw * cells.length,
            deliveredMw: power.deliveredMw * cells.length,
          },
          h3: cells.map((cell) => ({ h3Cell: cell.h3Cell, power, metadata })),
        }),
      ),
    );
    const header = Buffer.alloc(5);
    header.writeUInt32BE(body.length, 1);
    for (const client of clients) client.write(Buffer.concat([header, body]));
  }
  const server = createServer((request, response) => {
    response.setHeader("Access-Control-Allow-Origin", "*");
    response.setHeader("Access-Control-Allow-Headers", "*");
    response.setHeader("Access-Control-Allow-Methods", "POST, OPTIONS");
    if (request.method === "OPTIONS") {
      response.end();
      return;
    }
    response.setHeader("Content-Type", "application/connect+json");
    clients.add(response);
    response.on("close", () => clients.delete(response));
    publish();
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string")
    throw new Error("Stream address unavailable");
  await page.addInitScript((url) => {
    const nativeFetch = window.fetch;
    window.fetch = (input, options) => {
      const target = input instanceof Request ? input.url : String(input);
      return nativeFetch(
        target.endsWith("/gridos.v1.EventsService/WatchEvent") ? url : input,
        options,
      );
    };
  }, `http://127.0.0.1:${address.port}/`);
  return {
    eventId: event.eventId,
    cellCount: cells.length,
    publish,
    async close() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
  };
}
