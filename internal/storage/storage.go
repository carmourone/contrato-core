package storage

import (
	"context"
	"time"
)

type Store interface {
	Capabilities() CapSet
	Health(ctx context.Context) error
	Close() error

	Tenants() TenantRepo
	Types() TypeRepo
	Statuses() StatusRepo
	Properties() PropertyRepo
	ModelVersions() ModelVersionRepo

	Contracts() ContractRepo
	Objects() ObjectRepo
	Graph() GraphRepo
	Embeddings() EmbeddingRepo

	BeginTx(ctx context.Context, opts TxOptions) (Tx, error)
}

type Tx interface {
	Tenants() TenantRepo
	Types() TypeRepo
	Statuses() StatusRepo
	Properties() PropertyRepo
	ModelVersions() ModelVersionRepo
	Contracts() ContractRepo
	Objects() ObjectRepo
	Graph() GraphRepo
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type TxOptions struct {
	ReadOnly bool
	Timeout  time.Duration
}

type Page struct {
	Cursor string
	Limit  int
}

type PutOptions struct {
	ExpectedVersion *int
}

type Tenant struct {
	ID      string
	Name    string
	Version int
}

type TenantRepo interface {
	Create(ctx context.Context, name string) (Tenant, error)
	Get(ctx context.Context, id string) (Tenant, error)
	GetByName(ctx context.Context, name string) (Tenant, error)
}

type Type struct {
	TenantID string
	Domain   string
	Name     string
}

type TypeRepo interface {
	Create(ctx context.Context, t Type) (Type, error)
	GetByName(ctx context.Context, tenantID, domain, name string) (Type, error)
}

type Status struct {
	TenantID string
	Domain   string
	Name     string
}

type StatusRepo interface {
	Create(ctx context.Context, s Status) (Status, error)
	GetByName(ctx context.Context, tenantID, domain, name string) (Status, error)
}

type Property struct {
	TenantID     string
	OwnerType    string
	OwnerID      string // uuid
	Key          string
	ValueJSON    []byte
	ModelID      string // uuid
	ModelVersion int
	Version      int
}

type PropertyRepo interface {
	Put(ctx context.Context, p Property, opts PutOptions) (Property, error)
	Get(ctx context.Context, tenantID, ownerType, ownerID, key string) (Property, error)
}



type ModelVersion struct {
	TenantID   string
	ModelID    string // uuid stable id for a model lineage
	Version    int    // append-only version number within model_id
	Status     string // draft|enabled|disabled (FK constrained via statuses domain 'model_version')
	ChangeNote string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ModelVersionRepo interface {
	Create(ctx context.Context, mv ModelVersion) (ModelVersion, error) // v1 for new lineage, or next version for existing model_id
	Get(ctx context.Context, tenantID, modelID string, version int) (ModelVersion, error)
	GetLatestEnabled(ctx context.Context, tenantID string) (ModelVersion, error)
	List(ctx context.Context, tenantID string, modelID string, page Page) ([]ModelVersion, string, error)
}

type ContractRecord struct {
	TenantID     string
	ID           string // uuid
	Domain       string // 'contract'
	Type         string // constrained by types
	Status       string // constrained by statuses
	Action       string // allow|deny|defer|delegate|require_approval|escalate (FK via types domain contract_action)
	ModelID      string // uuid
	ModelVersion int
	Version      int
	Blob         []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ContractRepo interface {
	Get(ctx context.Context, tenantID, id string) (ContractRecord, error) // latest
	GetVersion(ctx context.Context, tenantID, id string, version int) (ContractRecord, error)
	Put(ctx context.Context, rec ContractRecord, opts PutOptions) (ContractRecord, error) // append-only
	ListByType(ctx context.Context, tenantID, domain, typ string, page Page) ([]ContractRecord, string, error)
}

type Object struct {
	TenantID  string
	ID        string // uuid
	NodeID    string // uuid (graph node stable id)
	Bytes     []byte
	ExtURL    string
	ETag      string
	Version   int
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ObjectPutOptions struct {
	IfMatchETag      string
	TTL              time.Duration
	ExpectedVersion  *int
}

type ObjectRepo interface {
	GetByNodeID(ctx context.Context, tenantID, nodeID string) (Object, error) // latest
	Put(ctx context.Context, obj Object, opts ObjectPutOptions) (Object, error) // append-only
	DeleteByNodeID(ctx context.Context, tenantID, nodeID string) error
}

type Node struct {
	TenantID     string
	ID           string // uuid
	Domain       string // 'node'
	Type         string // constrained by types
	ModelID      string // uuid
	ModelVersion int
	Version      int
	Blob         []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Edge struct {
	TenantID     string
	FromID       string // uuid
	ToID         string // uuid
	Domain       string // 'edge'
	Type         string // constrained by types
	ModelID      string // uuid
	ModelVersion int
	Version      int
	Blob         []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type GraphRepo interface {
	PutNode(ctx context.Context, n Node, opts PutOptions) (Node, error) // append-only
	GetNode(ctx context.Context, tenantID, id string) (Node, error) // latest
	PutEdge(ctx context.Context, e Edge, opts PutOptions) (Edge, error) // append-only
	OutEdges(ctx context.Context, tenantID, fromID, domain, typ string, page Page) ([]Edge, string, error)
}

type NodeMatch struct {
	Node
	Similarity float64 `json:"similarity"`
}

type EdgeMatch struct {
	Edge
	Similarity float64 `json:"similarity"`
}

type EmbeddingRepo interface {
	SetNodeEmbedding(ctx context.Context, tenantID, nodeID, model string, vec []float32) error
	SearchNodes(ctx context.Context, tenantID string, vec []float32, limit int) ([]NodeMatch, error)
	SetEdgeEmbedding(ctx context.Context, tenantID, fromID, toID, domain, typ, model string, vec []float32) error
	SearchEdges(ctx context.Context, tenantID string, vec []float32, limit int) ([]EdgeMatch, error)
}
