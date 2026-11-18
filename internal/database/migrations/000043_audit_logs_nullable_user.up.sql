-- System/job actions have no authenticated user; audit_logs.user_id is NULL for them.
ALTER TABLE audit_logs ALTER COLUMN user_id DROP NOT NULL;
