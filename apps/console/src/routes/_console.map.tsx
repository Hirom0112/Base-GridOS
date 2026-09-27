import { createFileRoute } from "@tanstack/react-router";
import { FleetMap } from "../map/map";

export const Route = createFileRoute("/_console/map")({ component: FleetMap });
