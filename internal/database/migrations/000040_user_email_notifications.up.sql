-- Per-user opt-out for notification emails (contract/payment reminders). Security
-- and account emails are always sent and ignore this preference.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_notifications BOOLEAN NOT NULL DEFAULT TRUE;
