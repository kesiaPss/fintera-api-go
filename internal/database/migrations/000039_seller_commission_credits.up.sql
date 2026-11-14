-- Recoverable commission credits created when an approved/signed contract with
-- an already-paid commission is cancelled. An admin applies the credit later.
CREATE TABLE IF NOT EXISTS seller_commission_credits (
    id BIGSERIAL PRIMARY KEY,
    seller_id BIGINT NOT NULL,
    contract_id BIGINT NOT NULL,
    amount NUMERIC(15,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ,
    settled_by BIGINT,
    CONSTRAINT fk_seller_commission_credits_seller FOREIGN KEY (seller_id) REFERENCES users(id),
    CONSTRAINT fk_seller_commission_credits_contract FOREIGN KEY (contract_id) REFERENCES contracts(id),
    CONSTRAINT fk_seller_commission_credits_settled_by FOREIGN KEY (settled_by) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_seller_commission_credits_seller_id ON seller_commission_credits (seller_id);
CREATE INDEX IF NOT EXISTS idx_seller_commission_credits_status ON seller_commission_credits (status);
