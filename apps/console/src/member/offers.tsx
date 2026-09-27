import { useState } from "react";
import { clone } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "../api/auth";
import {
  MemberOfferTermsSchema,
  type MemberOfferTerms,
  type SelectedMemberPlan,
  type ScheduledTravelFlexWindow,
} from "../api/gen/gridos/v1/member_pb";
import { offerTerms } from "./offer-terms";
import { OfferReview, money } from "./offer-review";
import { TravelSchedule } from "./travel-schedule";
import { TravelReturn } from "./travel-return";

export default function MemberOffers({
  memberId,
  currentPlan,
}: {
  memberId: string;
  currentPlan?: SelectedMemberPlan;
}) {
  const { client, identity } = useSession();
  const cache = useQueryClient();
  const [selected, setSelected] = useState<MemberOfferTerms | null>(null);
  const [completed, setCompleted] = useState(false);
  const query = useQuery({
    queryKey: ["member-offers", memberId, identity.role],
    queryFn: ({ signal }) =>
      client.member.listMemberOffers({ memberId }, { signal }),
    enabled: identity.role === "member",
  });
  function refresh() {
    void cache.invalidateQueries({ queryKey: ["member-status"] });
    void cache.invalidateQueries({ queryKey: ["member-offers", memberId] });
  }
  function confirmed() {
    setCompleted(true);
    refresh();
  }
  if (identity.role !== "member") return null;
  return (
    <section aria-label="Plans and Travel Flex">
      <h2>Plans and Travel Flex</h2>
      <button disabled={query.isFetching} onClick={() => query.refetch()}>
        Refresh offers and windows
      </button>
      {query.isPending && <p role="status">Loading household offers…</p>}
      {query.isError && (
        <p role="alert">
          Offers unavailable: {query.error.message}{" "}
          <button onClick={() => query.refetch()}>Retry offers</button>
        </p>
      )}
      {selected ? (
        <>
          <OfferReview
            memberId={memberId}
            terms={selected}
            onConfirmed={confirmed}
            onDismiss={() => setSelected(null)}
            travel={(offerId) => (
              <TravelSchedule
                memberId={memberId}
                terms={selected}
                offerId={offerId}
                onConfirmed={confirmed}
                onDismiss={() => setSelected(null)}
              />
            )}
          />
          {completed && (
            <button
              onClick={() => {
                setSelected(null);
                setCompleted(false);
              }}
            >
              Browse offers
            </button>
          )}
        </>
      ) : (
        query.data && (
          <>
            {!query.data.offers.length && (
              <p>No offers are available for this household.</p>
            )}
            <div className="member-offers">
              {query.data.offers.map((terms) => {
                const valid = offerTerms.safeParse(terms).success;
                const eligible =
                  terms.kind === 1 ||
                  (terms.memberPlanId === currentPlan?.memberPlanId &&
                    terms.catalogVersion === currentPlan.catalogVersion &&
                    terms.policyVersion === currentPlan.policyVersion &&
                    terms.temporaryReservePercent !== undefined &&
                    terms.temporaryReservePercent <=
                      currentPlan.reserveFloorPercent);
                return (
                  <article
                    className="member-offer"
                    key={`${terms.market}:${terms.catalogVersion}:${terms.memberPlanId}:${terms.kind}`}
                  >
                    <h3>
                      {terms.displayName}
                      {terms.kind === 2 ? " · Travel Flex" : ""}
                    </h3>
                    <p>{terms.priceText}</p>
                    <p>Plan backup reserve: {terms.reserveFloorPercent}%</p>
                    {!valid && (
                      <p>Complete, bounded offer terms are unavailable.</p>
                    )}
                    {!eligible && (
                      <p>
                        Choose a matching plan before scheduling this Travel
                        Flex offer.
                      </p>
                    )}
                    <button
                      disabled={!valid || !eligible}
                      onClick={() =>
                        setSelected(clone(MemberOfferTermsSchema, terms))
                      }
                    >
                      View {terms.displayName}{" "}
                      {terms.kind === 2 ? "Travel Flex" : "plan"}
                    </button>
                  </article>
                );
              })}
            </div>
          </>
        )
      )}
      {query.data && (
        <section aria-label="Scheduled Travel Flex">
          <h3>Scheduled Travel Flex</h3>
          {!query.data.travelFlexWindows.length && (
            <p>No scheduled Travel Flex windows.</p>
          )}
          {query.data.travelFlexWindows.map((window) => (
            <TravelWindow
              key={window.windowId}
              window={window}
              memberId={memberId}
              onConfirmed={refresh}
            />
          ))}
        </section>
      )}
    </section>
  );
}

const timestamp = z.object({
  seconds: z.bigint(),
  nanos: z.number().int().min(0).max(999999999),
});
const windowSchema = z
  .object({
    windowId: z.string().min(1),
    startTime: timestamp,
    endTime: timestamp,
    timezone: z
      .string()
      .min(1)
      .refine((zone) => {
        try {
          new Intl.DateTimeFormat("en", { timeZone: zone });
          return true;
        } catch {
          return false;
        }
      }),
    temporaryReservePercent: z.number().min(0).max(100),
    fixedCreditCents: z.bigint().min(0n),
    creditType: z.union([z.literal(1), z.literal(2), z.literal(3)]),
    consentVersion: z.string().min(1),
    earlyReturnAction: z.union([z.literal(1), z.literal(2)]),
    cancelledAt: timestamp.optional(),
  })
  .refine((value) => value.endTime.seconds > value.startTime.seconds);

function TravelWindow({
  window,
  memberId,
  onConfirmed,
}: {
  window: ScheduledTravelFlexWindow;
  memberId: string;
  onConfirmed: () => void;
}) {
  if (!windowSchema.safeParse(window).success)
    return (
      <p role="alert">
        A stored travel window has incomplete terms. Early return is unavailable
        for that window.
      </p>
    );
  const format = new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: window.timezone,
  });
  return (
    <article className="member-offer">
      <h4>
        {format.format(timestampDate(window.startTime!))} –{" "}
        {format.format(timestampDate(window.endTime!))}
      </h4>
      <p>
        {window.timezone} · Temporary reserve {window.temporaryReservePercent}%
        · Fixed credit {money(window.fixedCreditCents)}{" "}
        {new Map([
          [1, "daily"],
          [2, "per event"],
          [3, "annual"],
        ]).get(window.creditType)}
      </p>
      <TravelReturn
        memberId={memberId}
        window={window}
        onConfirmed={onConfirmed}
      />
    </article>
  );
}
