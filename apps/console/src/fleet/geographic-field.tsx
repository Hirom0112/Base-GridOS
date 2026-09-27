import { useMemo, type ComponentProps } from "react";
import { create } from "@bufbuild/protobuf";
import { H3SiteAggregateSchema } from "../api/gen/gridos/v1/api_pb";
import { useEventStream } from "../events/events-live";
import { useReplayClock } from "../events/replay-clock";
import { useGeographicCells } from "../map/geographic-time";
import LivingGrid from "./living-grid";

export default function GeographicField({
  eventId,
  ...props
}: ComponentProps<typeof LivingGrid> & { eventId?: string }) {
  const { position } = useReplayClock();
  const historical = useGeographicCells(position?.at ?? null);
  const { query } = useEventStream(position ? undefined : eventId);
  const cells = useMemo(
    () =>
      historical.data?.cells.map((cell) =>
        create(H3SiteAggregateSchema, {
          h3Cell: cell.h3Cell,
          siteCount: cell.siteCount,
          installedMw: { value: cell.installedMw, metadata: cell.metadata },
          installedMwh: { value: cell.installedMwh, metadata: cell.metadata },
        }),
      ) ?? [],
    [historical.data],
  );
  return (
    <>
      {position && historical.isPending && (
        <p role="status">Loading geography at the replay time…</p>
      )}
      {position && historical.isError && (
        <p role="alert">
          Historical geography unavailable: {historical.error.message}
        </p>
      )}
      <LivingGrid
        {...props}
        cells={position ? cells : props.cells}
        response={position || query.isError ? undefined : query.data?.at(-1)}
      />
    </>
  );
}
