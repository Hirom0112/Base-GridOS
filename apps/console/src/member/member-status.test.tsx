import { readFileSync } from "node:fs";
import { fromJsonString } from "@bufbuild/protobuf";
import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { GetMemberStatusResponseSchema } from "../api/gen/gridos/v1/member_pb";
import { MemberStatus } from "./member-status";

const memberId = "member-9d5ecb7e1fda8fcce5d0";
const siteId = "site_9d5ecb7e1fda8fcce5d0";
function fixture(variant = "on_grid") {
  return fromJsonString(
    GetMemberStatusResponseSchema,
    readFileSync(
      `../../testdata/fixtures/api/MemberService/GetMemberStatus.${variant}.json`,
      "utf8",
    ),
  );
}

test.each([
  ["on_grid", "On grid"],
  ["off_grid_outage", "Off-grid outage"],
  ["off_grid_no_home_power", "No home power"],
  ["off_grid_overcurrent", "Overcurrent"],
  ["off_grid_overcurrent_standby", "Overcurrent standby"],
  ["telemetry_unavailable", "Telemetry unavailable"],
])("member status preserves operating state %s", (variant, label) => {
  render(
    <MemberStatus
      data={fixture(variant)}
      memberId={memberId}
      siteId={siteId}
    />,
  );
  expect(screen.getByRole("heading", { name: label })).toBeVisible();
  expect(
    screen.getByText("Plan reserve floor").parentElement,
  ).toHaveTextContent("65%");
});

test("unavailable telemetry never presents stale charge as current", () => {
  render(
    <MemberStatus
      data={fixture("telemetry_unavailable")}
      memberId={memberId}
      siteId={siteId}
    />,
  );
  expect(screen.getByText("State of energy").parentElement).toHaveTextContent(
    "Unavailable",
  );
  expect(screen.queryByText("80%")).not.toBeInTheDocument();
});

test("online measurements retain units and plan charges", () => {
  render(<MemberStatus data={fixture()} memberId={memberId} siteId={siteId} />);
  expect(screen.getByText("80%")).toBeVisible();
  expect(screen.getByText("4 h")).toBeVisible();
  expect(screen.getByText("12 h")).toBeVisible();
  expect(screen.getByText(/19.99 USD/)).toBeVisible();
});

test("member status refuses another household response", () => {
  render(
    <MemberStatus data={fixture()} memberId="other-member" siteId={siteId} />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Household evidence is invalid",
  );
  expect(screen.queryByText("80%")).not.toBeInTheDocument();
});
