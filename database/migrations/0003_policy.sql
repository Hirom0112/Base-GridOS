BEGIN;

CREATE TABLE IF NOT EXISTS reserve_policies (
    policy_version text PRIMARY KEY,
    protected_hardware_floor_percent double precision NOT NULL CHECK (protected_hardware_floor_percent BETWEEN 0 AND 100),
    member_plan_floor_percent double precision NOT NULL CHECK (member_plan_floor_percent BETWEEN 0 AND 100),
    dynamic_override_percent double precision NOT NULL CHECK (dynamic_override_percent BETWEEN 0 AND 100),
    effective_reserve_percent double precision NOT NULL CHECK (effective_reserve_percent BETWEEN 0 AND 100),
    effective_at timestamptz NOT NULL,
    expires_at timestamptz,
    correlation_id text NOT NULL,
    CHECK (expires_at IS NULL OR expires_at > effective_at),
    CHECK (effective_reserve_percent = GREATEST(protected_hardware_floor_percent, member_plan_floor_percent, dynamic_override_percent))
);

CREATE TABLE IF NOT EXISTS resilience_plans (
    resilience_plan_id text PRIMARY KEY,
    member_id text NOT NULL,
    market text NOT NULL,
    reserve_floor_percent double precision NOT NULL CHECK (reserve_floor_percent BETWEEN 0 AND 100),
    consent_text text NOT NULL,
    consent_version text NOT NULL,
    policy_version text NOT NULL REFERENCES reserve_policies(policy_version),
    effective_at timestamptz NOT NULL,
    expires_at timestamptz,
    correlation_id text NOT NULL,
    CHECK (expires_at IS NULL OR expires_at > effective_at),
    UNIQUE (member_id, policy_version)
);

CREATE TABLE IF NOT EXISTS travel_flex_windows (
    travel_flex_window_id text PRIMARY KEY,
    member_id text NOT NULL,
    start_time timestamptz NOT NULL,
    end_time timestamptz NOT NULL,
    timezone text NOT NULL,
    temporary_reserve_percent double precision NOT NULL CHECK (temporary_reserve_percent BETWEEN 0 AND 100),
    early_return_action text NOT NULL CHECK (early_return_action IN ('RESTORE_PLAN_RESERVE', 'RESTORE_MAXIMUM_RESERVE')),
    credit_type text NOT NULL CHECK (credit_type IN ('FIXED_DAILY', 'FIXED_EVENT', 'FIXED_ANNUAL')),
    credit_cents bigint NOT NULL,
    consent_text text NOT NULL,
    consent_version text NOT NULL,
    policy_version text NOT NULL REFERENCES reserve_policies(policy_version),
    correlation_id text NOT NULL,
    CHECK (end_time > start_time)
);

CREATE TABLE IF NOT EXISTS reserve_overrides (
    reserve_override_id text PRIMARY KEY,
    member_id text NOT NULL,
    reason text NOT NULL CHECK (reason IN ('WEATHER', 'OUTAGE_RISK', 'HEALTH', 'STALE_TELEMETRY', 'ALARM', 'EARLY_RETURN')),
    reserve_floor_percent double precision NOT NULL CHECK (reserve_floor_percent BETWEEN 0 AND 100),
    effective_at timestamptz NOT NULL,
    expires_at timestamptz,
    policy_version text NOT NULL REFERENCES reserve_policies(policy_version),
    correlation_id text NOT NULL,
    CHECK (expires_at IS NULL OR expires_at > effective_at)
);

CREATE TABLE IF NOT EXISTS pricing_catalog_snapshots (
    catalog_version text NOT NULL,
    member_plan_id text NOT NULL,
    market text NOT NULL,
    energy_plan jsonb NOT NULL,
    energy_term_months integer NOT NULL CHECK (energy_term_months >= 0),
    energy_monthly_charge_cents bigint NOT NULL,
    battery_plan jsonb NOT NULL,
    battery_term_months integer NOT NULL CHECK (battery_term_months >= 0),
    battery_monthly_charge_cents bigint NOT NULL,
    flexibility_reward_cents bigint NOT NULL,
    effective_at timestamptz NOT NULL,
    expires_at timestamptz,
    correlation_id text NOT NULL,
    PRIMARY KEY (catalog_version, member_plan_id),
    CHECK (expires_at IS NULL OR expires_at > effective_at)
);

CREATE TABLE IF NOT EXISTS plan_add_ons (
    catalog_version text NOT NULL,
    member_plan_id text NOT NULL,
    name text NOT NULL,
    rate_offset_cents bigint NOT NULL,
    monthly_fee_cents bigint NOT NULL,
    correlation_id text NOT NULL,
    PRIMARY KEY (catalog_version, member_plan_id, name),
    FOREIGN KEY (catalog_version, member_plan_id) REFERENCES pricing_catalog_snapshots(catalog_version, member_plan_id)
);

CREATE TABLE IF NOT EXISTS flexibility_offers (
    offer_id text PRIMARY KEY,
    member_id text NOT NULL,
    catalog_version text NOT NULL,
    contract_version text NOT NULL,
    flexibility_reward_cents bigint NOT NULL,
    price_text text NOT NULL,
    effective_at timestamptz NOT NULL,
    expires_at timestamptz,
    correlation_id text NOT NULL,
    CHECK (expires_at IS NULL OR expires_at > effective_at)
);

CREATE TABLE IF NOT EXISTS reward_ledger (
    entry_id text PRIMARY KEY,
    member_id text NOT NULL,
    event_id text REFERENCES dispatch_events(event_id),
    offer_id text REFERENCES flexibility_offers(offer_id),
    amount_cents bigint NOT NULL,
    entry_type text NOT NULL,
    recorded_at timestamptz NOT NULL,
    correlation_id text NOT NULL
);

DROP TRIGGER IF EXISTS reward_ledger_append_only ON reward_ledger;
CREATE TRIGGER reward_ledger_append_only
BEFORE UPDATE OR DELETE ON reward_ledger
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
