import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";
import { AnomalyAlerts } from "./anomaly-alert";
import { MemberStatus } from "./member-status";
import "../events/report.css";
import "./member.css";

const householdSchema = z.object({
  memberId: z.string().trim().min(1).max(200),
  siteId: z.string().trim().min(1).max(200),
});

export function MemberHome() {
  const { client, identity } = useSession();
  const [household, setHousehold] = useState<z.infer<
    typeof householdSchema
  > | null>(null);
  const [error, setError] = useState("");
  const query = useQuery({
    queryKey: ["member-status", household, identity.role],
    queryFn: ({ signal }) => {
      if (!household) throw new Error("Select a household");
      return client.member.getMemberStatus(household, { signal });
    },
    enabled: household !== null,
    refetchInterval: 5000,
  });
  function open(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const result = householdSchema.safeParse(
      Object.fromEntries(new FormData(event.currentTarget)),
    );
    if (!result.success) {
      setError("Enter your member and site IDs.");
      return;
    }
    setError("");
    setHousehold(result.data);
  }
  if (identity.mode !== "local")
    return (
      <main className="auth-gate">
        <h1>Home energy</h1>
        <p>Your household connection is not available for this session.</p>
      </main>
    );
  return (
    <div className="console member-shell" data-theme="light">
      <main className="member-home">
        <header>
          <p className="eyebrow">GridOS · Member</p>
          <h1>Home energy</h1>
        </header>
        <form className="report-comparison" onSubmit={open}>
          <label>
            Member ID
            <input name="memberId" defaultValue={identity.memberId} required />
          </label>
          <label>
            Site ID
            <input name="siteId" required />
          </label>
          <button className="secondary-button" type="submit">
            Open household
          </button>
        </form>
        {error && <p role="alert">{error}</p>}
        {query.isFetching && <p role="status">Refreshing household status…</p>}
        {query.isError && (
          <p role="alert">
            Household status unavailable: {query.error.message}{" "}
            <button onClick={() => query.refetch()}>Retry household</button>
          </p>
        )}
        {query.data && household && (
          <MemberStatus
            data={query.data}
            memberId={household.memberId}
            siteId={household.siteId}
          />
        )}
        {household && (
          <AnomalyAlerts
            key={household.memberId}
            memberId={household.memberId}
          />
        )}
      </main>
    </div>
  );
}
