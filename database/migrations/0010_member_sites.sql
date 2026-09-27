BEGIN;

CREATE TABLE IF NOT EXISTS member_sites (
    site_id text PRIMARY KEY,
    member_id text NOT NULL,
    bound_at timestamptz NOT NULL,
    source text NOT NULL CHECK (source IN ('SIMULATED', 'MEMBER_AUTHORIZED')),
    provenance jsonb NOT NULL,
    CHECK (provenance->>'provenance' = source)
);

CREATE INDEX IF NOT EXISTS member_sites_member_id_idx ON member_sites (member_id);

COMMIT;
