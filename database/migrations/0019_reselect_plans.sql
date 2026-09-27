BEGIN;

ALTER TABLE resilience_plans DROP CONSTRAINT resilience_plans_member_id_policy_version_key;

WITH ordered AS (
    SELECT resilience_plan_id,
           lead(effective_at) OVER (PARTITION BY member_id ORDER BY effective_at, resilience_plan_id) AS next_effective_at
    FROM resilience_plans
)
UPDATE resilience_plans AS plan
SET expires_at = ordered.next_effective_at
FROM ordered
WHERE plan.resilience_plan_id = ordered.resilience_plan_id
  AND ordered.next_effective_at IS NOT NULL
  AND (plan.expires_at IS NULL OR plan.expires_at > ordered.next_effective_at);

CREATE UNIQUE INDEX resilience_plans_one_current_per_member
ON resilience_plans(member_id) WHERE expires_at IS NULL;

COMMIT;
