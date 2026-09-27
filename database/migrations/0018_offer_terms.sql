BEGIN;

CREATE TABLE offer_terms (
    catalog_version text NOT NULL,
    member_plan_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('PLAN', 'TRAVEL_FLEX')),
    policy_version text NOT NULL REFERENCES reserve_policies(policy_version),
    contract_version text NOT NULL CHECK (length(trim(contract_version)) > 0),
    consent_version text NOT NULL CHECK (length(trim(consent_version)) > 0),
    consent_text text NOT NULL CHECK (length(trim(consent_text)) > 0),
    price_text text NOT NULL CHECK (length(trim(price_text)) > 0),
    temporary_reserve_percent double precision,
    credit_type text,
    fixed_credit_cents bigint NOT NULL CHECK (fixed_credit_cents >= 0),
    PRIMARY KEY (catalog_version, member_plan_id, kind),
    FOREIGN KEY (catalog_version, member_plan_id) REFERENCES pricing_catalog_snapshots(catalog_version, member_plan_id),
    CHECK ((kind = 'PLAN' AND temporary_reserve_percent IS NULL AND credit_type IS NULL AND fixed_credit_cents = 0)
        OR (kind = 'TRAVEL_FLEX' AND temporary_reserve_percent BETWEEN 0 AND 100
            AND credit_type IN ('FIXED_DAILY', 'FIXED_EVENT', 'FIXED_ANNUAL')))
);

CREATE TRIGGER offer_terms_append_only
BEFORE UPDATE OR DELETE ON offer_terms
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
