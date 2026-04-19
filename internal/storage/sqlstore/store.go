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
