import { createFileRoute } from "@tanstack/react-router";
import { ReportComparison } from "../events/comparison";

export const Route = createFileRoute("/_console/events/compare")({
  component: ReportComparison,
});
