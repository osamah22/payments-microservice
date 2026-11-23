package wallet

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/osamah22/payments-microservice/internal/platform/apperr"
	"github.com/osamah22/payments-microservice/internal/wallet/walletdb"
)

var (
	ErrInsufficientFunds = apperr.NewRule("insufficient_funds", "insufficient funds")
)

func (s *Store) Transfer(ctx context.Context, from, to uuid.UUID, amount int64, currency string) (Transaction, error) {
	var posted Transaction
	err := s.inTx(ctx, func(q *walletdb.Queries) error {
		// Lock both accounts until we commit. Another transfer that touches
		// either one waits right here, so the balance below stays true.
		ids := []uuid.UUID{from, to}
		slices.SortFunc(ids, func(a, b uuid.UUID) int {
			return bytes.Compare(a[:], b[:])
		})
		for _, id := range ids {
			if _, err := q.LockAccount(ctx, id); err != nil {
				return fmt.Errorf("lock account %s: %w", id, err)
			}
		}
		b, err := q.GetAccountBalance(ctx, from)
		if err != nil {
			return fmt.Errorf("get balance %s: %w", from, err)
		}
		if b.Balance < amount {
			return fmt.Errorf("wallet %s: %w", from, ErrInsufficientFunds)
		}
		posted, err = insertTransaction(ctx, q, NewTransaction{
			Description: "transfer",
			Postings: []Posting{
				{AccountID: from, Direction: Debit, Amount: amount, Currency: currency},
				{AccountID: to, Direction: Credit, Amount: amount, Currency: currency},
			},
		})
		return err
	})
	if err != nil {
		return Transaction{}, fmt.Errorf("transfer: %w", err)
	}
	return posted, nil
}
