# Questions and decisions

Running log kept by the director. Every decision made without asking Hirom is
written here the moment it is made, with the reason. Questions that genuinely
need Hirom appear under **Open** and move down once answered. This file
replaces the `ASSUMPTIONS.md` the build order originally named; spec-gap
assumptions reported by workers land here too.

## Open

(none)

## 2026-09-26

- **Q:** Who orchestrates the build? **D:** The Codex session named worker
  orchestrates and spawns up to six subagents; Claude directs, verifies every
  item independently, and marks `[x]`. Channel: `codex queue` in, `.local/mailbox.log` out.
- **Q:** Branches and worktrees, or one tree? **D:** One shared tree, direct
  to `main`, commits by exact path. Lane ownership already keeps paths
  disjoint, so branches would only add a merge step. Hirom preferred simpler.
- **Q:** How much gating per commit? **D:** Only the fast pre-commit hook on
  staged files plus the one verify command for the item in hand. The full
  suite runs once per wave by the director. Hirom does not want the build
  bogged down in repeated gates.
- **Q:** Where do the console screens get built? **D:** In a separate UI
  track (`UI_TRACK.md`) run by Hirom's own agent, which owns `apps/console`.
  Backend lane F supplies fixtures, a mock server, ingest, streaming, and
  integration tests instead.
- **Q:** Do backend wave gates block on the UI track's Playwright spec?
  **D:** No. If the UI track has not reached that gate's steps yet, the gate
  report records "UI track pending" and the next backend wave starts. The
  Playwright results are attached when they arrive.
- **Q:** Wave 0 sequencing when the toolchain is missing? **D:** Lane A's
  item 0A.1 (install Go, buf, uv with Python 3.12, sqlc, temporal, bazelisk)
  runs first as a single agent for a few minutes. The other five lanes start
  as soon as it reports done. Lanes may write their RED tests before that but
  cannot verify without the tools.
- **Q:** Where do assumptions and decisions live? **D:** Here, in one file,
  instead of a root `ASSUMPTIONS.md`. Fewer root files. `STUBS.md` stays at
  the root because the global rules name it.
- **Q:** What happens to the 57 MB `presentations/` folder? **D:** Kept on
  disk, ignored by git. The research doc moved to `docs/design/visual-system.md`.
- **Q:** Commit subject style? **D:** Plain imperative, no `type(scope):`
  prefix, enforced by the commit-msg hook. Matches the umath reference and
  the global rule that an unenforced convention is drift.
- **Q:** Measurement boundary for the canonical event? **D:** Meter net
  export by default, battery terminal selectable, because FULL_SPEC §15
  leaves it open and the worked physics example uses net export.
- **Q:** Critical-load definition for backup duration? **D:** Always report
  hours at current usage and hours at a 750 W reference load. Settles the
  FULL_SPEC §15 question for the first build.
- **Q:** Bazel now or later? **D:** Wave 5. Native `go test`, `pytest`, and
  `pnpm` verify everything until the tree is real.
- **Q:** Local auth for the offline demo? **D:** `GRIDOS_AUTH_MODE=local`
  with a dev identity, marked STUBBED, because the judged path may not call
  Clerk over the network.
