package sqlstore

import (
	"context"
	"database/sql"
)

func scanNullInt(row *sql.Row) (int, error) {
	var v sql.NullInt64
	if err := row.Scan(&v); err != nil { return 0, err }
	if !v.Valid { return 0, nil }
	return int(v.Int64), nil
}

func LatestVersion(ctx context.Context, q querier, table, tenantID, keyCol, keyVal string) (int, error) {
	row := q.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM "+table+" WHERE tenant_id=$1 AND "+keyCol+"=$2", tenantID, keyVal)
	return scanNullInt(row)
}
