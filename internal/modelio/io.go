package modelio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"contrato/internal/storage"
)

type StoreWithDB interface {
	storage.Store
	DB() *sql.DB
}

type Options struct {
	WithContracts bool

	TenantName     string
	OutputPath     string
	InputPath      string
	EnableImported bool
	ChangeNote     string
}

func ExportLatestEnabled(ctx context.Context, st StoreWithDB, opts Options) error {
	if opts.TenantName == "" {
		return errors.New("tenant name required")
	}
	if opts.OutputPath == "" {
		return errors.New("output path required")
	}

	tenant, err := st.Tenants().GetByName(ctx, opts.TenantName)
	if err != nil { return err }

	mv, err := st.ModelVersions().GetLatestEnabled(ctx, tenant.ID)
	if err != nil { return err }

	db := st.DB()

	b := Bundle{
		FormatVersion: 1,
		Tenant:        Tenant{Name: opts.TenantName},
		Model:         Model{ModelID: mv.ModelID, Version: mv.Version, Status: mv.Status, ChangeNote: mv.ChangeNote, CreatedAt: mv.CreatedAt},
	}

	tRows, err := db.QueryContext(ctx, `SELECT domain, name FROM types WHERE tenant_id=$1 ORDER BY domain, name`, tenant.ID)
	if err != nil { return err }
	defer tRows.Close()
	for tRows.Next() {
		var d, n string
		if err := tRows.Scan(&d, &n); err != nil { return err }
		b.Types = append(b.Types, Type{Domain: d, Name: n})
	}
	if err := tRows.Err(); err != nil { return err }

	sRows, err := db.QueryContext(ctx, `SELECT domain, name FROM statuses WHERE tenant_id=$1 ORDER BY domain, name`, tenant.ID)
	if err != nil { return err }
	defer sRows.Close()
	for sRows.Next() {
		var d, n string
		if err := sRows.Scan(&d, &n); err != nil { return err }
		b.Statuses = append(b.Statuses, Status{Domain: d, Name: n})
	}
	if err := sRows.Err(); err != nil { return err }

	nRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, id)
  id, domain, type, blob
FROM graph_nodes
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, id, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil { return err }
	defer nRows.Close()
	for nRows.Next() {
		var id, domain, typ string
		var blob []byte
		if err := nRows.Scan(&id, &domain, &typ, &blob); err != nil { return err }
		var bm map[string]any
		_ = json.Unmarshal(blob, &bm)
		b.Nodes = append(b.Nodes, Node{ID: id, Domain: domain, Type: typ, Blob: bm})
	}
	if err := nRows.Err(); err != nil { return err }

	eRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, from_id, to_id, domain, type)
  from_id, to_id, domain, type, blob
FROM graph_edges
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, from_id, to_id, domain, type, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil { return err }
	defer eRows.Close()
	for eRows.Next() {
		var fromID, toID, domain, typ string
		var blob []byte
		if err := eRows.Scan(&fromID, &toID, &domain, &typ, &blob); err != nil { return err }
		var bm map[string]any
		_ = json.Unmarshal(blob, &bm)
		b.Edges = append(b.Edges, Edge{FromID: fromID, ToID: toID, Domain: domain, Type: typ, Blob: bm})
	}
	if err := eRows.Err(); err != nil { return err }

	pRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, owner_type, owner_id, key)
  owner_type, owner_id, key, value
FROM properties
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, owner_type, owner_id, key, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil { return err }
	defer pRows.Close()
	for pRows.Next() {
		var ot, oid, key string
		var vj []byte
		if err := pRows.Scan(&ot, &oid, &key, &vj); err != nil { return err }
		var vm map[string]any
		_ = json.Unmarshal(vj, &vm)
		b.Properties = append(b.Properties, Property{OwnerType: ot, OwnerID: oid, Key: key, Value: vm})
	}
	if err := pRows.Err(); err != nil { return err }

	if opts.WithContracts {
		reasons, err := loadReasons(ctx, db, tenant.ID)
		if err != nil { return err }
		obligations, err := loadObligations(ctx, db, tenant.ID)
		if err != nil { return err }

		cRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, id)
  id, domain, type, status, action, model_id, model_version, version, blob
FROM contracts
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, id, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
		if err != nil { return err }
		defer cRows.Close()
		for cRows.Next() {
			var id, domain, typ, status, modelID string
			var action sql.NullString
			var modelVersion, version int
			var blob []byte
			if err := cRows.Scan(&id, &domain, &typ, &status, &action, &modelID, &modelVersion, &version, &blob); err != nil {
				return err
			}
			var bm map[string]any
			_ = json.Unmarshal(blob, &bm)
			k := contractKey(id, version)
			c := Contract{
				ID: id, Domain: domain, Type: typ, Status: status,
				ModelID: modelID, ModelVersion: modelVersion, Blob: bm,
				Reasons: reasons[k], Obligations: obligations[k],
			}
			if action.Valid { c.Action = action.String }
			b.Contracts = append(b.Contracts, c)
		}
		if err := cRows.Err(); err != nil { return err }
	}

	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil { return err }
	return os.WriteFile(opts.OutputPath, raw, 0644)
}

func ImportBundle(ctx context.Context, st StoreWithDB, opts Options) error {
	if opts.InputPath == "" {
		return errors.New("input path required")
	}
	raw, err := os.ReadFile(opts.InputPath)
	if err != nil { return err }

	var b Bundle
	if err := json.Unmarshal(raw, &b); err != nil { return err }
	if b.FormatVersion == 0 {
		b.FormatVersion = 1
	}

	tenantName := opts.TenantName
	if tenantName == "" {
		tenantName = b.Tenant.Name
	}
	if tenantName == "" {
		return errors.New("tenant name missing (use --tenant or bundle.tenant.name)")
	}

	t, err := st.Tenants().GetByName(ctx, tenantName)
	if err != nil {
		created, err2 := st.Tenants().Create(ctx, tenantName)
		if err2 != nil { return err2 }
		t = created
	}

	for _, s := range b.Statuses {
		if _, err := st.Statuses().Create(ctx, storage.Status{TenantID: t.ID, Domain: s.Domain, Name: s.Name}); err != nil {
			return fmt.Errorf("create status %s:%s: %w", s.Domain, s.Name, err)
		}
	}
	for _, ty := range b.Types {
		if _, err := st.Types().Create(ctx, storage.Type{TenantID: t.ID, Domain: ty.Domain, Name: ty.Name}); err != nil {
			return fmt.Errorf("create type %s:%s: %w", ty.Domain, ty.Name, err)
		}
	}

	mvStatus := "draft"
	if b.Model.Status != "" { mvStatus = b.Model.Status }
	if opts.EnableImported { mvStatus = "enabled" }
	changeNote := opts.ChangeNote
	if changeNote == "" { changeNote = b.Model.ChangeNote }

	mv, err := st.ModelVersions().Create(ctx, storage.ModelVersion{
		TenantID:   t.ID,
		ModelID:    b.Model.ModelID,
		Status:     mvStatus,
		ChangeNote: changeNote,
	})
	if err != nil { return err }

	for _, n := range b.Nodes {
		blob, _ := json.Marshal(n.Blob)
		if _, err := st.Graph().PutNode(ctx, storage.Node{
			TenantID:     t.ID,
			ID:           n.ID,
			Domain:       firstNonEmpty(n.Domain, "node"),
			Type:         n.Type,
			ModelID:      mv.ModelID,
			ModelVersion: mv.Version,
			Blob:         blob,
		}, storage.PutOptions{}); err != nil {
			return fmt.Errorf("put node: %w", err)
		}
	}

	for _, e := range b.Edges {
		blob, _ := json.Marshal(e.Blob)
		if _, err := st.Graph().PutEdge(ctx, storage.Edge{
			TenantID:     t.ID,
			FromID:       e.FromID,
			ToID:         e.ToID,
			Domain:       firstNonEmpty(e.Domain, "edge"),
			Type:         e.Type,
			ModelID:      mv.ModelID,
			ModelVersion: mv.Version,
			Blob:         blob,
		}, storage.PutOptions{}); err != nil {
			return fmt.Errorf("put edge: %w", err)
		}
	}

	for _, p := range b.Properties {
		val, _ := json.Marshal(p.Value)
		if _, err := st.Properties().Put(ctx, storage.Property{
			TenantID:     t.ID,
			OwnerType:    p.OwnerType,
			OwnerID:      p.OwnerID,
			Key:          p.Key,
			ValueJSON:    val,
			ModelID:      mv.ModelID,
			ModelVersion: mv.Version,
		}, storage.PutOptions{}); err != nil {
			return fmt.Errorf("put property: %w", err)
		}
	}

	if opts.WithContracts {
		for _, c := range b.Contracts {
			blob, _ := json.Marshal(c.Blob)
			if _, err := st.Contracts().Put(ctx, storage.ContractRecord{
				TenantID:     t.ID,
				ID:           c.ID,
				Domain:       firstNonEmpty(c.Domain, "contract"),
				Type:         c.Type,
				Status:       c.Status,
				Action:       c.Action,
				ModelID:      mv.ModelID,
				ModelVersion: mv.Version,
				Blob:         blob,
			}, storage.PutOptions{}); err != nil {
				return fmt.Errorf("put contract: %w", err)
			}
		}
	}

	return nil
}

func loadReasons(ctx context.Context, db *sql.DB, tenantID string) (map[string][]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT contract_id, contract_version, code FROM contract_reasons WHERE tenant_id=$1`, tenantID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var cid, code string
		var cv int
		if err := rows.Scan(&cid, &cv, &code); err != nil { return nil, err }
		k := contractKey(cid, cv)
		out[k] = append(out[k], code)
	}
	return out, rows.Err()
}

func loadObligations(ctx context.Context, db *sql.DB, tenantID string) (map[string][]map[string]any, error) {
	rows, err := db.QueryContext(ctx, `SELECT contract_id, contract_version, obligation, params FROM contract_obligations WHERE tenant_id=$1`, tenantID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := map[string][]map[string]any{}
	for rows.Next() {
		var cid, ob string
		var cv int
		var params []byte
		if err := rows.Scan(&cid, &cv, &ob, &params); err != nil { return nil, err }
		var pm map[string]any
		_ = json.Unmarshal(params, &pm)
		k := contractKey(cid, cv)
		out[k] = append(out[k], map[string]any{"name": ob, "params": pm})
	}
	return out, rows.Err()
}

func contractKey(id string, version int) string {
	return fmt.Sprintf("%s:%d", id, version)
}

func firstNonEmpty(v, def string) string {
	if v != "" { return v }
	return def
}
