package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/practicesubjectcatalog"
	"github.com/motifpath/core-domain/internal/domain"
)

// practiceContentVersionID is the id of the content version's one row.
const practiceContentVersionID = 1

// EntPracticeSubjectCatalogRepository keeps the practice subject catalog and
// the content version via ent/Postgres.
type EntPracticeSubjectCatalogRepository struct {
	client *ent.Client
}

func NewEntPracticeSubjectCatalogRepository(client *ent.Client) *EntPracticeSubjectCatalogRepository {
	return &EntPracticeSubjectCatalogRepository{client: client}
}

func (r *EntPracticeSubjectCatalogRepository) Catalogs(ctx context.Context, scopes []string) (map[string]domain.PracticeSubjectCatalog, int64, error) {
	version, err := contentVersion(ctx, r.client)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.client.PracticeSubjectCatalog.Query().Where(practicesubjectcatalog.ScopeIn(scopes...)).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	found := make(map[string]domain.PracticeSubjectCatalog, len(rows))
	for _, row := range rows {
		catalog, err := toDomainPracticeSubjectCatalog(row)
		if err != nil {
			return nil, 0, err
		}
		found[row.Scope] = catalog
	}
	return found, version, nil
}

// BumpContentVersion creates the version's row at 1 the first time.
func (r *EntPracticeSubjectCatalogRepository) BumpContentVersion(ctx context.Context) error {
	_, err := r.client.ExecContext(ctx,
		`INSERT INTO practice_content_versions (id, version) VALUES ($1, 1)
		 ON CONFLICT (id) DO UPDATE SET version = practice_content_versions.version + 1`,
		practiceContentVersionID)
	return err
}

// Store reads the version under a share lock, so a raise at the same time
// waits for its commit.
func (r *EntPracticeSubjectCatalogRepository) Store(ctx context.Context, version int64, catalogs []domain.PracticeSubjectCatalog) (bool, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return false, err
	}
	stored, err := storeIn(ctx, tx, version, catalogs)
	if err != nil || !stored {
		return false, rollback(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func storeIn(ctx context.Context, tx *ent.Tx, version int64, catalogs []domain.PracticeSubjectCatalog) (bool, error) {
	current, err := lockedContentVersion(ctx, tx)
	if err != nil || current != version {
		return false, err
	}
	scopes := make([]string, len(catalogs))
	creates := make([]*ent.PracticeSubjectCatalogCreate, len(catalogs))
	for i, c := range catalogs {
		items, err := json.Marshal(toCatalogItemRows(c.Items))
		if err != nil {
			return false, fmt.Errorf("practice subject catalog %q items: %w", c.Scope, err)
		}
		scopes[i] = c.Scope
		creates[i] = tx.PracticeSubjectCatalog.Create().SetScope(c.Scope).SetContentVersion(c.ContentVersion).
			SetFormatVersion(c.FormatVersion).SetBuiltAt(c.BuiltAt).SetItems(items)
	}
	if _, err := tx.PracticeSubjectCatalog.Delete().Where(practicesubjectcatalog.ScopeIn(scopes...)).Exec(ctx); err != nil {
		return false, err
	}
	if err := tx.PracticeSubjectCatalog.CreateBulk(creates...).Exec(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// contentVersion is the current content version, 0 before its first raise.
func contentVersion(ctx context.Context, client *ent.Client) (int64, error) {
	row, err := client.PracticeContentVersion.Get(ctx, practiceContentVersionID)
	if ent.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.Version, nil
}

// lockedContentVersion is the current content version, read under a share
// lock held until tx ends; 0 before its first raise.
func lockedContentVersion(ctx context.Context, tx *ent.Tx) (int64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT version FROM practice_content_versions WHERE id = $1 FOR SHARE", practiceContentVersionID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var version int64
	if rows.Next() {
		if err := rows.Scan(&version); err != nil {
			return 0, err
		}
	}
	return version, rows.Err()
}

// catalogItemRow is how a catalog item is stored in a row's items JSON.
type catalogItemRow struct {
	ItemKey string          `json:"item_key"`
	NodeIDs []string        `json:"node_ids"`
	InHand  bool            `json:"in_hand"`
	InHead  bool            `json:"in_head"`
	Rung    string          `json:"rung,omitempty"`
	Ways    []catalogWayRow `json:"ways,omitempty"`
}

type catalogWayRow struct {
	Way  string `json:"way"`
	Rung string `json:"rung"`
}

func toCatalogItemRows(items []domain.PracticeSubjectCatalogItem) []catalogItemRow {
	rows := make([]catalogItemRow, len(items))
	for i, item := range items {
		rows[i] = catalogItemRow{ItemKey: item.ItemKey, NodeIDs: item.NodeIDs, InHand: item.InHand, InHead: item.InHead, Rung: string(item.Rung)}
		for _, w := range item.Ways {
			rows[i].Ways = append(rows[i].Ways, catalogWayRow{Way: w.Way, Rung: string(w.Rung)})
		}
	}
	return rows
}

func toDomainPracticeSubjectCatalog(row *ent.PracticeSubjectCatalog) (domain.PracticeSubjectCatalog, error) {
	var stored []catalogItemRow
	if err := json.Unmarshal(row.Items, &stored); err != nil {
		return domain.PracticeSubjectCatalog{}, fmt.Errorf("practice subject catalog %q items: %w", row.Scope, err)
	}
	items := make([]domain.PracticeSubjectCatalogItem, len(stored))
	for i, s := range stored {
		items[i] = domain.PracticeSubjectCatalogItem{ItemKey: s.ItemKey, NodeIDs: s.NodeIDs, InHand: s.InHand, InHead: s.InHead, Rung: domain.PracticeRung(s.Rung)}
		for _, w := range s.Ways {
			items[i].Ways = append(items[i].Ways, domain.PracticeSubjectCatalogWay{Way: w.Way, Rung: domain.PracticeRung(w.Rung)})
		}
	}
	return domain.PracticeSubjectCatalog{
		Scope: row.Scope, ContentVersion: row.ContentVersion, FormatVersion: row.FormatVersion, BuiltAt: row.BuiltAt.UTC(), Items: items,
	}, nil
}
