-- payer_wallet: the on-chain address the user actually paid from. Distinct from
-- wallet_address (the wallet they want signals about), since the wallet that
-- funds a subscription is often a different account (e.g. a CEX withdrawal
-- address) than the one being tracked. Populated by the payment verifier from
-- the tx's `from` field.
ALTER TABLE subscribers
    ADD COLUMN IF NOT EXISTS payer_wallet TEXT;
