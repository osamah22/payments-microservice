package wallet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osamah22/payments-microservice/internal/platform/apperr"
	"github.com/osamah22/payments-microservice/internal/wallet/walletdb"
)

type (
	Account        = walletdb.Account
	AccountBalance = walletdb.AccountBalance
	Entry          = walletdb.Entry
	NewAccount     = walletdb.CreateAccountParams
	Direction      = walletdb.EntryDirection
)

const (
	Debit  = walletdb.EntryDirectionDebit
	Credit = walletdb.EntryDirectionCredit

	KindSystem = walletdb.AccountKindSystem
	KindWallet = walletdb.AccountKindWallet
)

var (
	ErrAccountNotFound = fmt.Errorf("account %w", apperr.ErrNotFound)
	ErrAccountExists   = fmt.Errorf("%w: account already exists", apperr.ErrConflict)
)

type Store struct {
	pool *pgxpool.Pool
	q    *walletdb.Queries
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: walletdb.New(pool)}
}

// CreateAccount opens a new account.
func (s *Store) CreateAccount(ctx context.Context, a NewAccount) (Account, error) {
	acc, err := s.q.CreateAccount(ctx, a)
	if isUniqueViolation(err) {
		return Account{}, ErrAccountExists
	}
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	return acc, nil
}

// SystemAccount returns one of our own accounts by name and currency, like the
// settlement account for USD.
func (s *Store) SystemAccount(ctx context.Context, name, currency string) (Account, error) {
	acc, err := s.q.GetSystemAccount(ctx, walletdb.GetSystemAccountParams{Name: name, Currency: currency})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get system account %s %s: %w", name, currency, err)
	}
	return acc, nil
}

// Account returns the account with the given id.
func (s *Store) Account(ctx context.Context, id uuid.UUID) (Account, error) {
	acc, err := s.q.GetAccountByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("get account %s: %w", id, err)
	}
	return acc, nil
}

// Balance returns the account's balance derived from its entries by the
// account_balances view.
func (s *Store) Balance(ctx context.Context, id uuid.UUID) (AccountBalance, error) {
	b, err := s.q.GetAccountBalance(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountBalance{}, ErrAccountNotFound
	}
	if err != nil {
		return AccountBalance{}, fmt.Errorf("get balance %s: %w", id, err)
	}
	return b, nil
}

func (s *Store) Entries(ctx context.Context, id uuid.UUID, before int64, limit int32) ([]Entry, error) {
	if before <= 0 {
		before = math.MaxInt64
	}
	entries, err := s.q.ListEntriesByAccount(ctx, walletdb.ListEntriesByAccountParams{
		AccountID: id,
		BeforeID:  before,
		PageSize:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list entries %s: %w", id, err)
	}
	return entries, nil
}

// isUniqueViolation reports weather err is Postgres rejecting duplicates.
func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505" // unique_violation
}
