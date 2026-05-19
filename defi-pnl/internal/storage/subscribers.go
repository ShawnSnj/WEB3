package storage

import "fmt"

// ListActiveSubscriberChatIDs returns telegram_chat_id for every subscriber
// whose subscription window covers today and whose status is 'active'. The
// date comparison uses ::date so a row whose start_at/expire_at carries an
// arbitrary intra-day timestamp still qualifies for the whole calendar day.
func ListActiveSubscriberChatIDs() ([]int64, error) {
	rows, err := DB.Query(`
        SELECT telegram_chat_id
        FROM subscribers
        WHERE start_at::date  <= CURRENT_DATE
          AND expire_at::date >= CURRENT_DATE
          AND status = 'active'
          AND telegram_chat_id IS NOT NULL
    `)
	if err != nil {
		return nil, fmt.Errorf("list active subscribers: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// UpsertSubscriberOnStart records a /start interaction. If the chat is new it
// is inserted with status='inactive'; if it already exists, status is left
// alone (so a previously paid/active user can't be silently demoted by
// re-issuing /start) but wallet_address is back-filled when missing — that
// way an old row inserted before the wallet_address wiring still gets its
// audit trail populated on the next /start.
//
// `paymentWallet` is the receiving address shown to the user in the /start
// message. Empty string is treated as NULL so we don't store '' rows.
func UpsertSubscriberOnStart(chatID int64, paymentWallet string) error {
	_, err := DB.Exec(`
        INSERT INTO subscribers (telegram_chat_id, wallet_address, status)
        VALUES ($1, NULLIF($2, ''), 'inactive')
        ON CONFLICT (telegram_chat_id) DO UPDATE
        SET wallet_address = COALESCE(subscribers.wallet_address, EXCLUDED.wallet_address)
    `, chatID, paymentWallet)
	if err != nil {
		return fmt.Errorf("upsert subscriber: %w", err)
	}
	return nil
}

// SavePaymentTx attaches a tx hash submitted via /paid to the subscriber row.
// If the chat hasn't sent /start yet we still create the row so the verifier
// has something to inspect. We never override an already-active subscription;
// pending payments move the row from 'inactive' to 'pending' so the verifier
// has a clear queue, but an existing 'active' status is preserved.
//
// `paymentWallet` is the receiving address the user was instructed to send to
// (the operator's PAYMENT_WALLET at the time of payment). It is denormalized
// onto the row so a future PAYMENT_WALLET rotation doesn't lose the audit
// trail of "where did we tell this user to send funds?".
//
// `updated_at` is bumped to NOW() only when the submitted tx_hash actually
// differs from what is already on file — re-submitting the same hash is not
// considered a state change.
func SavePaymentTx(chatID int64, txHash, paymentWallet string) error {
	_, err := DB.Exec(`
        INSERT INTO subscribers (telegram_chat_id, tx_hash, wallet_address, status, updated_at)
        VALUES ($1, $2, $3, 'pending', NOW())
        ON CONFLICT (telegram_chat_id) DO UPDATE
        SET tx_hash        = EXCLUDED.tx_hash,
            wallet_address = EXCLUDED.wallet_address,
            status         = CASE
                                 WHEN subscribers.status = 'active' THEN subscribers.status
                                 ELSE 'pending'
                             END,
            updated_at     = CASE
                                 WHEN subscribers.tx_hash IS DISTINCT FROM EXCLUDED.tx_hash THEN NOW()
                                 ELSE subscribers.updated_at
                             END
    `, chatID, txHash, paymentWallet)
	if err != nil {
		return fmt.Errorf("save payment tx: %w", err)
	}
	return nil
}
