package sqlstore

import (
	"context"
	"database/sql"

	"contrato/internal/storage"
)

type Store struct {
	db  *sql.DB
	cap storage.CapSet
}

func (s *Store) DB() *sql.DB { return s.db }

func NewStore(db *sql.DB, caps storage.CapSet) *Store { return &Store{db: db, cap: caps} }

func (s *Store) Capabilities() storage.CapSet     { return s.cap }
func (s *Store) Health(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) Close() error                     { return s.db.Close() }

func (s *Store) Tenants() storage.TenantRepo             { return &TenantsRepo{q: s.db} }
func (s *Store) Types() storage.TypeRepo                 { return &TypesRepo{q: s.db} }
func (s *Store) Statuses() storage.StatusRepo            { return &StatusesRepo{q: s.db} }
func (s *Store) Properties() storage.PropertyRepo        { return &PropertiesRepo{q: s.db} }
func (s *Store) ModelVersions() storage.ModelVersionRepo { return &ModelVersionsRepo{q: s.db} }
func (s *Store) Contracts() storage.ContractRepo         { return &ContractsRepo{q: s.db} }
func (s *Store) Objects() storage.ObjectRepo             { return &ObjectsRepo{q: s.db} }
func (s *Store) Graph() storage.GraphRepo                { return &GraphRepo{q: s.db} }

func (s *Store) BeginTx(ctx context.Context, opts storage.TxOptions) (storage.Tx, error) {
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: opts.ReadOnly})
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, err
	}
	return &Tx{tx: tx, cap: s.cap, cancel: cancel}, nil
}
