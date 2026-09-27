import { useQuery } from "@tanstack/react-query";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { useSession } from "../api/auth";

export function verifyGeographicTime(
  response: { asOf?: Timestamp },
  requested: Timestamp | undefined,
) {
  if (!requested) return;
  if (
    response.asOf?.seconds !== requested.seconds ||
    response.asOf.nanos !== requested.nanos
  )
    throw new Error(
      "Geographic evidence does not match the requested replay time.",
    );
}

export function useGeographicCells(time: Timestamp | "current" | null) {
  const { client, identity } = useSession();
  const asOf = typeof time === "object" ? (time ?? undefined) : undefined;
  return useQuery({
    queryKey: [
      "geo-cells",
      identity.role,
      7,
      asOf?.seconds.toString(),
      asOf?.nanos,
    ],
    enabled: time !== null && identity.role !== "member",
    queryFn: async ({ signal }) => {
      const response = await client.geo.listCells(
        { resolution: 7, loadZones: ["LZ_AEN"], asOf },
        { signal },
      );
      verifyGeographicTime(response, asOf);
      for (const cell of response.cells) verifyGeographicTime(cell, asOf);
      return response;
    },
    refetchInterval: time === "current" ? 5000 : false,
  });
}
