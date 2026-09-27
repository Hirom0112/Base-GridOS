import { createFileRoute } from "@tanstack/react-router";
import { RegionalContext } from "../context/regional";
import { FleetMetrics, useFleet } from "../fleet/fleet";

export const Route = createFileRoute("/_console/fleet")({
  component: FleetPage,
});

function FleetPage() {
  const { summary } = useFleet();
  return (
    <>
      <FleetMetrics summary={summary.data?.summary} />
      <RegionalContext />
    </>
  );
}
