BEGIN;

CREATE TABLE IF NOT EXISTS sites (
    site_id text PRIMARY KEY,
    weather_zone text NOT NULL,
    load_zone text NOT NULL,
    h3_cell text NOT NULL,
    has_solar boolean NOT NULL,
    has_automatic_backup boolean NOT NULL,
    provenance text NOT NULL CHECK (provenance IN ('CONFIRMED_PUBLIC', 'CONFIRMED_SANDBOX', 'AUTHORIZED_OPERATIONAL', 'DERIVED', 'SIMULATED')),
    simulation_seed bigint,
    correlation_id text NOT NULL
);

CREATE TEMPORARY TABLE fleet_seed (
    record jsonb NOT NULL
) ON COMMIT DROP;

\copy fleet_seed (record) FROM 'testdata/fleets/texas-50.jsonl'

INSERT INTO sites (
    site_id,
    weather_zone,
    load_zone,
    h3_cell,
    has_solar,
    has_automatic_backup,
    provenance,
    simulation_seed,
    correlation_id
)
SELECT
    record->>'site_id',
    record->>'weather_zone',
    record->>'load_zone',
    record->>'h3_cell',
    (record->>'has_solar')::boolean,
    (record->>'has_automatic_backup')::boolean,
    record->>'provenance',
    (record->>'simulation_seed')::bigint,
    'seed:texas-50'
FROM fleet_seed
ON CONFLICT (site_id) DO UPDATE
SET weather_zone = EXCLUDED.weather_zone,
    load_zone = EXCLUDED.load_zone,
    h3_cell = EXCLUDED.h3_cell,
    has_solar = EXCLUDED.has_solar,
    has_automatic_backup = EXCLUDED.has_automatic_backup,
    provenance = EXCLUDED.provenance,
    simulation_seed = EXCLUDED.simulation_seed,
    correlation_id = EXCLUDED.correlation_id;

INSERT INTO risk_policy (
    version, effective_at, expires_at, outage_probability_threshold,
    telemetry_freshness_seconds, gateway_cadence_seconds, weather_floor_percent,
    outage_floor_percent, stale_floor_percent, alarm_floor_percent,
    communications_floor_percent, health_floor_percent, weather_zone_ugc, provenance
) VALUES (
    'risk-policy-sim-1', '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z', 0.01,
    30, 15, 60, 60, 40, 100, 40, 100,
    '{"austin-5000.jsonl":{"SCENT":{"ugc":["TXZ192"],"same":["048453"]}}}',
    '{"provenance":"SIMULATED","weather_zone_ugc":"SIMULATED"}'
) ON CONFLICT (version) DO NOTHING;

INSERT INTO reserve_policies (
    policy_version, protected_hardware_floor_percent, member_plan_floor_percent,
    dynamic_override_percent, effective_reserve_percent, effective_at, expires_at,
    correlation_id, provenance
) VALUES
    ('reserve-sim-essential-1', 10, 10, 0, 10, '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z', 'seed:SIMULATED:catalog-sim-1', '{"provenance":"SIMULATED"}'),
    ('reserve-sim-balanced-1', 10, 30, 0, 30, '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z', 'seed:SIMULATED:catalog-sim-1', '{"provenance":"SIMULATED"}'),
    ('reserve-sim-maximum-1', 10, 60, 0, 60, '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z', 'seed:SIMULATED:catalog-sim-1', '{"provenance":"SIMULATED"}')
ON CONFLICT (policy_version) DO NOTHING;

INSERT INTO pricing_catalog_snapshots (
    catalog_version, member_plan_id, market, display_name, reserve_floor_percent,
    energy_plan, energy_term_months, energy_monthly_charge_cents,
    battery_plan, battery_term_months, battery_monthly_charge_cents,
    flexibility_reward_cents, effective_at, expires_at, correlation_id, provenance
) VALUES
    ('catalog-sim-1', 'essential', 'ERCOT', 'Essential', 10, '{"provenance":"SIMULATED","price_text":"SIMULATED","contract_version":"sim-1"}', 0, 0, '{"provenance":"SIMULATED"}', 0, 0, 500, '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z', 'seed:SIMULATED:catalog-sim-1', '{"provenance":"SIMULATED"}'),
    ('catalog-sim-1', 'balanced', 'ERCOT', 'Balanced', 30, '{"provenance":"SIMULATED","price_text":"SIMULATED","contract_version":"sim-1"}', 0, 0, '{"provenance":"SIMULATED"}', 0, 0, 800, '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z', 'seed:SIMULATED:catalog-sim-1', '{"provenance":"SIMULATED"}'),
    ('catalog-sim-1', 'maximum', 'ERCOT', 'Maximum', 60, '{"provenance":"SIMULATED","price_text":"SIMULATED","contract_version":"sim-1"}', 0, 0, '{"provenance":"SIMULATED"}', 0, 0, 1200, '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z', 'seed:SIMULATED:catalog-sim-1', '{"provenance":"SIMULATED"}')
ON CONFLICT (catalog_version, member_plan_id) DO NOTHING;

INSERT INTO offer_terms (
    catalog_version, member_plan_id, kind, policy_version, contract_version, consent_version,
    consent_text, price_text, temporary_reserve_percent, credit_type, fixed_credit_cents
)
SELECT catalog_version, member_plan_id, 'PLAN', 'reserve-sim-' || member_plan_id || '-1', 'sim-contract-1', 'sim-consent-1',
    'SIMULATED: I accept the ' || display_name || ' plan terms',
    'SIMULATED: ' || display_name || ' plan, $0 monthly energy and battery charges',
    NULL, NULL, 0
FROM pricing_catalog_snapshots WHERE catalog_version = 'catalog-sim-1'
UNION ALL
SELECT catalog_version, member_plan_id, 'TRAVEL_FLEX', 'reserve-sim-' || member_plan_id || '-1', 'sim-contract-1', 'sim-flex-consent-1',
    'SIMULATED: I accept temporary Travel Flex reserve terms',
    'SIMULATED: fixed daily Travel Flex credit', 20, 'FIXED_DAILY', 500
FROM pricing_catalog_snapshots WHERE catalog_version = 'catalog-sim-1'
ON CONFLICT (catalog_version, member_plan_id, kind) DO NOTHING;

COMMIT;
