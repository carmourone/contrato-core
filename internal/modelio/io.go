package modelio

import (
	"context"
	"database/sql"
	"encoding/base64"
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

	// types
	tr, err := db.QueryContext(ctx, `SELECT domain, name FROM types WHERE tenant_id=$1 ORDER BY domain, name`, tenant.ID)
	if err != nil { return err }
	for tr.Next() {
		var d, n string
		if err := tr.Scan(&d, &n); err != nil { tr.Close(); return err }
		b.Types = append(b.Types, Type{Domain: d, Name: n})
	}
	tr.Close()

	// statuses
	sr, err := db.QueryContext(ctx, `SELECT domain, name FROM statuses WHERE tenant_id=$1 ORDER BY domain, name`, tenant.ID)
	if err != nil { return err }
	for sr.Next() {
		var d, n string
		if err := sr.Scan(&d, &n); err != nil { sr.Close(); return err }
		b.Statuses = append(b.Statuses, Status{Domain: d, Name: n})
	}
	sr.Close()

	// nodes latest for this model version
	nr, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, id)
  id, domain, type, blob
FROM graph_nodes
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, id, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil { return err }
	for nr.Next() {
		var id, domain, typ string
		var blob []byte
		if err := nr.Scan(&id, &domain, &typ, &blob); err != nil { nr.Close(); return err }
		var bm map[string]any
		_ = json.Unmarshal(blob, &bm)
		b.Nodes = append(b.Nodes, Node{ID: id, Domain: domain, Type: typ, Blob: bm})
	}
	nr.Close()

	// edges latest for this model version
	er, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, id)
  from_id, to_id, domain, type, blob
FROM graph_edges
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, id, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil { return err }
	for er.Next() {
		var fromID, toID, domain, typ string
		var blob []byte
		if err := er.Scan(&fromID, &toID, &domain, &typ, &blob); err != nil { er.Close(); return err }
		var bm map[string]any
		_ = json.Unmarshal(blob, &bm)
		b.Edges = append(b.Edges, Edge{FromID: fromID, ToID: toID, Domain: domain, Type: typ, Blob: bm})
	}
	er.Close()

	// properties latest for this model version
	pr, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, owner_type, owner_id, key)
  owner_type, owner_id, key, value
FROM properties
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, owner_type, owner_id, key, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil { return err }
	for pr.Next() {
		var ot, oid, key string
		var vj []byte
		if err := pr.Scan(&ot, &oid, &key, &vj); err != nil { pr.Close(); return err }
		var vm map[string]any
		_ = json.Unmarshal(vj, &vm)
		b.Properties = append(b.Properties, Property{OwnerType: ot, OwnerID: oid, Key: key, Value: vm})
	}
	pr.Close()

	// contracts (optional)
	if opts.WithContracts {
		cr, err := db.QueryContext(ctx, `
SELECT tenant_id, id, domain, type, status, action, model_id, model_version, version, blob
FROM contracts
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY id, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
		if err != nil { return err }
		defer cr.Close()

		// Fetch reasons and obligations keyed by (id, version)
		reasons := map[string][]string{}
		rr, err := db.QueryContext(ctx, `
SELECT contract_id, contract_version, code
FROM contract_reasons
WHERE tenant_id=$1
`, tenant.ID)
		if err == nil {
			for rr.Next() {
				var cid string
				var cv int
				var code string
				if err := rr.Scan(&cid, &cv, &code); err != nil { rr.Close(); return err }
				k := cid + ":" + fmt.Sprintf("%d", cv)
				reasons[k] = append(reasons[k], code)
			}
			rr.Close()
		}

		obligations := map[string][]map[string]any{}
		orows, err := db.QueryContext(ctx, `
SELECT contract_id, contract_version, obligation, params
FROM contract_obligations
WHERE tenant_id=$1
`, tenant.ID)
		if err == nil {
			for orows.Next() {
				var cid string
				var cv int
				var ob string
				var params []byte
				if err := orows.Scan(&cid, &cv, &ob, &params); err != nil { orows.Close(); return err }
				var pm map[string]any
				_ = json.Unmarshal(params, &pm)
				k := cid + ":" + fmt.Sprintf("%d", cv)
				obligations[k] = append(obligations[k], map[string]any{"name": ob, "params": pm})
			}
			orows.Close()
		}

		for cr.Next() {
			var tid, id, domain, typ, status, action, modelID string
			var modelVersion, version int
			var blob []byte
			if err := cr.Scan(&tid, &id, &domain, &typ, &status, &action, &modelID, &modelVersion, &version, &blob); err != nil { return err }
			var bm map[string]any
			_ = json.Unmarshal(blob, &bm)
			k := id + ":" + fmt.Sprintf("%d", version)
			b.Contracts = append(b.Contracts, Contract{
				ID: id, Domain: domain, Type: typ, Status: status, Action: action,
				ModelID: modelID, ModelVersion: modelVersion, Blob: bm,
				Reasons: reasons[k], Obligations: obligations[k],
			})
		}
	}

	// (properties merged into properties)

	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil { return err }
	return os.WriteFile(opts.OutputPath, raw, 0644)
}

func ImportBundle(ctx context.Context, st StoreWithDB, opts Options) error {
	if opts.InputPath == "" {
		return errors.New("input path required")
	}
	raw, err := os.ReadFile(opts.InputPath)
	if err != nil {
		return err
	}

	var b Bundle
	if err := json.Unmarshal(raw, &b); err != nil {
		return err
	}
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

	// ensure tenant exists
	t, err := st.Tenants().GetByName(ctx, tenantName)
	if err != nil {
		created, err2 := st.Tenants().Create(ctx, tenantName)
		if err2 != nil {
			return err2
		}
		t = created
	}

	// Ensure registry values exist (types + statuses)
	for _, s := range b.Statuses {
		_, _ = st.Statuses().Create(ctx, storage.Status{TenantID: t.ID, Domain: s.Domain, Name: s.Name})
	}
	for _, ty := range b.Types {
		_, _ = st.Types().Create(ctx, storage.Type{TenantID: t.ID, Domain: ty.Domain, Name: ty.Name})
	}

	// Create a new model_version
	mvStatus := "draft"
	if b.Model.Status != "" {
		mvStatus = b.Model.Status
	}
	if opts.EnableImported {
		mvStatus = "enabled"
	}
	changeNote := opts.ChangeNote
	if changeNote == "" {
		changeNote = b.Model.ChangeNote
	}

	mv, err := st.ModelVersions().Create(ctx, storage.ModelVersion{
		TenantID:   t.ID,
		ModelID:    b.Model.ModelID, // optional: keep lineage if provided
		Status:     mvStatus,
		ChangeNote: changeNote,
	})
	if err != nil {
		return err
	}

	// Insert nodes/edges/properties/metrics referencing this model version.
	for _, n := range b.Nodes {
		blob, _ := json.Marshal(n.Blob)
		_, err := st.Graph().PutNode(ctx, storage.Node{
			TenantID:     t.ID,
			ID:           n.ID,
			Domain:       firstNonEmpty(n.Domain, "node"),
			Type:         n.Type,
			ModelID:      mv.ModelID,
			ModelVersion: mv.Version,
			Blob:         blob,
		}, storage.PutOptions{})
		if err != nil {
			return fmt.Errorf("put node: %w", err)
		}
	}
	for _, e := range b.Edges {
		blob, _ := json.Marshal(e.Blob)
		_, err := st.Graph().PutEdge(ctx, storage.Edge{
			TenantID:     t.ID,
			FromID:       e.FromID,
			ToID:         e.ToID,
			Domain:       firstNonEmpty(e.Domain, "edge"),
			Type:         e.Type,
			ModelID:      mv.ModelID,
			ModelVersion: mv.Version,
			Blob:         blob,
		}, storage.PutOptions{})
		if err != nil {
			return fmt.Errorf("put edge: %w", err)
		}
	}
	for _, p := range b.Properties {
		val, _ := json.Marshal(p.Value)
		_, err := st.Properties().Put(ctx, storage.Property{
			TenantID:     t.ID,
			OwnerType:    p.OwnerType,
			OwnerID:      p.OwnerID,
			Key:          p.Key,
			ValueJSON:    val,
			ModelID:      mv.ModelID,
			ModelVersion: mv.Version,
		}, storage.PutOptions{})
		if err != nil {
			return fmt.Errorf("put property: %w", err)
		}
	}
	for _, m := range append(b.properties, metricsToproperties(b.properties)...) {
		var jb []byte
		if m.JSON != nil {
			jb, _ = json.Marshal(*m.JSON)
		}
		_, err := st.properties().Put(ctx, storage.Property{
			TenantID:     t.ID,
			OwnerType:    m.OwnerType,
			OwnerID:      m.OwnerID,
			Key:          m.Key,
			Float:        m.Float,
			Int:          m.Int,
			Text:         m.Text,
			JSON:         jb,
			ModelID:      mv.ModelID,
			ModelVersion: mv.Version,
		}, storage.PutOptions{})
		if err != nil {
			return fmt.Errorf("put metric: %w", err)
		}
	}

	return nil
}

func exportBundle(ctx context.Context, db *sql.DB, tenant storage.Tenant, mv storage.ModelVersion) (Bundle, error) {
	b := Bundle{
		FormatVersion: 1,
		Tenant:        Tenant{Name: tenant.Name},
		Model: Model{
			ModelID:    mv.ModelID,
			Version:    mv.Version,
			Status:     mv.Status,
			ChangeNote: mv.ChangeNote,
			CreatedAt:  mv.CreatedAt,
		},
	}

	// types
	tRows, err := db.QueryContext(ctx, `SELECT domain, name FROM types WHERE tenant_id=$1 ORDER BY domain, name`, tenant.ID)
	if err != nil {
		return Bundle{}, err
	}
	defer tRows.Close()
	for tRows.Next() {
		var d, n string
		if err := tRows.Scan(&d, &n); err != nil {
			return Bundle{}, err
		}
		b.Types = append(b.Types, Type{Domain: d, Name: n})
	}

	// statuses
	sRows, err := db.QueryContext(ctx, `SELECT domain, name FROM statuses WHERE tenant_id=$1 ORDER BY domain, name`, tenant.ID)
	if err != nil {
		return Bundle{}, err
	}
	defer sRows.Close()
	for sRows.Next() {
		var d, n string
		if err := sRows.Scan(&d, &n); err != nil {
			return Bundle{}, err
		}
		b.Statuses = append(b.Statuses, Status{Domain: d, Name: n})
	}

	// nodes latest for this model
	nRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, id)
  id, domain, type, blob
FROM graph_nodes
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, id, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil {
		return Bundle{}, err
	}
	defer nRows.Close()
	for nRows.Next() {
		var id, domain, typ string
		var blob []byte
		if err := nRows.Scan(&id, &domain, &typ, &blob); err != nil {
			return Bundle{}, err
		}
		var m map[string]any
		_ = json.Unmarshal(blob, &m)
		b.Nodes = append(b.Nodes, Node{ID: id, Domain: domain, Type: typ, Blob: m})
	}

	// edges latest for this model
	eRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, from_id, to_id, domain, type)
  from_id, to_id, domain, type, blob
FROM graph_edges
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, from_id, to_id, domain, type, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil {
		return Bundle{}, err
	}
	defer eRows.Close()
	for eRows.Next() {
		var fromID, toID, domain, typ string
		var blob []byte
		if err := eRows.Scan(&fromID, &toID, &domain, &typ, &blob); err != nil {
			return Bundle{}, err
		}
		var m map[string]any
		_ = json.Unmarshal(blob, &m)
		b.Edges = append(b.Edges, Edge{FromID: fromID, ToID: toID, Domain: domain, Type: typ, Blob: m})
	}

	// properties latest for this model
	pRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, owner_type, owner_id, key)
  owner_type, owner_id, key, value
FROM properties
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, owner_type, owner_id, key, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil {
		return Bundle{}, err
	}
	defer pRows.Close()
	for pRows.Next() {
		var ot, oid, key string
		var val []byte
		if err := pRows.Scan(&ot, &oid, &key, &val); err != nil {
			return Bundle{}, err
		}
		var m map[string]any
		_ = json.Unmarshal(val, &m)
		b.Properties = append(b.Properties, Property{OwnerType: ot, OwnerID: oid, Key: key, Value: m})
	}

	// metrics latest for this model
	mRows, err := db.QueryContext(ctx, `
SELECT DISTINCT ON (tenant_id, owner_type, owner_id, key)
  owner_type, owner_id, key, value_float, value_int, value_text, value_json
FROM properties
WHERE tenant_id=$1 AND model_id=$2 AND model_version=$3
ORDER BY tenant_id, owner_type, owner_id, key, version DESC
`, tenant.ID, mv.ModelID, mv.Version)
	if err != nil {
		return Bundle{}, err
	}
	defer mRows.Close()

for mRows.Next() {
	var ot, oid, key string
	var vf sql.NullFloat64
	var vi sql.NullInt64
	var vt sql.NullString
	var vj []byte
	var vb []byte
	if err := mRows.Scan(&ot, &oid, &key, &vf, &vi, &vt, &vj, &vb); err != nil {
		return Bundle{}, err
	}
	pp := Property{OwnerType: ot, OwnerID: oid, Key: key}
	if vf.Valid {
		pp.Float = &vf.Float64
	}
	if vi.Valid {
		vv := vi.Int64
		pp.Int = &vv
	}
	if vt.Valid {
		vv := vt.String
		pp.Text = &vv
	}
	if vj != nil {
		var jm map[string]any
		_ = json.Unmarshal(vj, &jm)
		jr := jsonRaw(jm)
		pp.JSON = &jr
	}
	if vb != nil {
		enc := base64.StdEncoding.EncodeToString(vb)
		pp.BytesB64 = &enc
	}
	b.properties = append(b.properties, pp)
}
	}

	return b, nil
}

func firstNonEmpty(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
