BEGIN;

DROP TRIGGER IF EXISTS audit_journal_append_only ON audit_journal;
DROP TRIGGER IF EXISTS emergency_stops_append_only ON emergency_stops;
DROP TRIGGER IF EXISTS operator_approvals_append_only ON operator_approvals;
DROP TABLE IF EXISTS audit_journal;
DROP TABLE IF EXISTS verification_summaries;
DROP TABLE IF EXISTS emergency_stops;
DROP TABLE IF EXISTS operator_approvals;

COMMIT;
