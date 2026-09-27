BEGIN;

DROP INDEX IF EXISTS resilience_plans_one_current_per_member;

ALTER TABLE resilience_plans
ADD CONSTRAINT resilience_plans_member_id_policy_version_key UNIQUE (member_id, policy_version);

COMMIT;
