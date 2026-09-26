import { create } from "@bufbuild/protobuf";
import { render, screen, waitFor } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import {
  H3SiteAggregateSchema,
  FleetQuantityAggregateSchema,
} from "../api/gen/gridos/v1/api_pb";
import LivingGrid from "./living-grid";

const engine = vi.hoisted(() => ({
  update: vi.fn(),
  dispose: vi.fn(),
  highlight: vi.fn(),
}));
const mount = vi.hoisted(() => vi.fn());
vi.mock("./renderer", () => ({ mountGrid: mount.mockReturnValue(engine) }));

const cell = create(H3SiteAggregateSchema, {
  h3Cell: "87489d884ffffff",
  siteCount: 12n,
  installedMw: {
    value: 0.1,
    metadata: {
      timestamp: { seconds: 1786575300n },
      freshness: {},
      provenanceMix: [{ provenance: 5, recordCount: 12n }],
    },
  },
});

test("refreshes geography without replacing the renderer or losing selection", async () => {
  const select = vi.fn();
  const { rerender, unmount } = render(
    <LivingGrid cells={[cell]} selected={cell.h3Cell} onSelect={select} />,
  );
  await screen.findByText("3D geographic field");
  expect(mount).toHaveBeenCalledTimes(1);
  const updated = create(H3SiteAggregateSchema, {
    ...cell,
    installedMw: create(FleetQuantityAggregateSchema, {
      metadata: cell.installedMw?.metadata,
      value: 0.2,
    }),
  });
  rerender(
    <LivingGrid cells={[updated]} selected={cell.h3Cell} onSelect={select} />,
  );
  await waitFor(() => expect(screen.getByText("0.200")).toBeInTheDocument());
  expect(mount).toHaveBeenCalledTimes(1);
  expect(engine.dispose).not.toHaveBeenCalled();
  expect(engine.update).toHaveBeenLastCalledWith(
    expect.arrayContaining([
      expect.objectContaining({ id: cell.h3Cell, height: 3.6 }),
    ]),
    cell.h3Cell,
  );
  rerender(<LivingGrid cells={[]} selected={null} onSelect={select} />);
  await waitFor(() => expect(engine.update).toHaveBeenLastCalledWith([], null));
  expect(mount).toHaveBeenCalledTimes(1);
  unmount();
  expect(engine.dispose).toHaveBeenCalledTimes(1);
});
