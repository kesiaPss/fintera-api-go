-- Payments are repeatedly loaded per contract and ordered by due date by the
-- reminder jobs, payment history, and contract detail views.
CREATE INDEX IF NOT EXISTS idx_payments_contract_due_date
    ON payments(contract_id, due_date);
