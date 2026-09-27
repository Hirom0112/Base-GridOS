import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/_console/member")({
  component: MemberAccess,
});

function MemberAccess() {
  return (
    <section className="event-panel">
      <h2>Member access</h2>
      <p>Use a member session to open your household status and plan.</p>
    </section>
  );
}
