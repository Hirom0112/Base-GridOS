import { createFileRoute } from "@tanstack/react-router";
import { EventView } from "../events/event";

export const Route = createFileRoute("/_console/events/$eventId/report")({
  component: Page,
});
function Page() {
  return <EventView eventId={Route.useParams().eventId} view="report" />;
}
