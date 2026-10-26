package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/motifpath/core-domain/internal/adapters/repo/ent"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/practicecatalogsignal"
	"github.com/motifpath/core-domain/internal/adapters/repo/ent/practicesubjectcatalog"
	"github.com/motifpath/core-domain/internal/domain"
)

// practiceCatalogRebuildLock is the key of the transaction-scoped advisory
// lock a rebuild holds, so no two instances rebuild at once.
const practiceCatalogRebuildLock = 6_400_064

// EntPracticeSubjectCatalogRepository keeps the practice subject catalog and
// its rebuild queue via ent/Postgres.
type EntPracticeSubjectCatalogRepository struct {
	client *ent.Client
	now    func() time.Time
}

func NewEntPracticeSubjectCatalogRepository(client *ent.Client, now func() time.Time) *EntPracticeSubjectCatalogRepository {
	return &EntPracticeSubjectCatalogRepository{client: client, now: now}
}

func (r *EntPracticeSubjectCatalogRepository) Catalogs(ctx context.Context, scopes []string) (map[string]domain.PracticeSubjectCatalog, error) {
	rows, err := r.client.PracticeSubjectCatalog.Query().Where(practicesubjectcatalog.ScopeIn(scopes...)).All(ctx)
	if err != nil {
		return nil, err
	}
	found := make(map[string]domain.PracticeSubjectCatalog, len(rows))
	for _, row := range rows {
		catalog, err := toDomainPracticeSubjectCatalog(row)
		if err != nil {
			return nil, err
		}
		found[row.Scope] = catalog
	}
	return found, nil
}

func (r *EntPracticeSubjectCatalogRepository) Signal(ctx context.Context, reason domain.PracticeCatalogSignalReason) error {
	return r.client.PracticeCatalogSignal.Create().SetReason(string(reason)).SetCreatedAt(r.now()).Exec(ctx)
}

// Rebuild holds a transaction-scoped advisory lock while it builds, so a
// second instance asking at the same time finds it taken and builds nothing.
func (r *EntPracticeSubjectCatalogRepository) Rebuild(ctx context.Context, build func(context.Context) ([]domain.PracticeSubjectCatalog, error)) (bool, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return false, err
	}
	rebuilt, err := r.rebuildIn(ctx, tx, build)
	if err != nil || !rebuilt {
		return false, rollback(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *EntPracticeSubjectCatalogRepository) rebuildIn(ctx context.Context, tx *ent.Tx, build func(context.Context) ([]domain.PracticeSubjectCatalog, error)) (bool, error) {
	locked, err := tryAdvisoryXactLock(ctx, tx, practiceCatalogRebuildLock)
	if err != nil || !locked {
		return false, err
	}
	last, err := tx.PracticeCatalogSignal.Query().Order(ent.Desc(practicecatalogsignal.FieldID)).First(ctx)
	if ent.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	catalogs, err := build(ctx)
	if err != nil {
		return false, err
	}
	if _, err := tx.PracticeSubjectCatalog.Delete().Exec(ctx); err != nil {
		return false, err
	}
	creates := make([]*ent.PracticeSubjectCatalogCreate, len(catalogs))
	for i, c := range catalogs {
		items, err := json.Marshal(toCatalogItemRows(c.Items))
		if err != nil {
			return false, fmt.Errorf("practice subject catalog %q items: %w", c.Scope, err)
		}
		creates[i] = tx.PracticeSubjectCatalog.Create().SetScope(c.Scope).SetFormatVersion(c.FormatVersion).SetBuiltAt(c.BuiltAt).SetItems(items)
	}
	if err := tx.PracticeSubjectCatalog.CreateBulk(creates...).Exec(ctx); err != nil {
		return false, err
	}
	if _, err := tx.PracticeCatalogSignal.Delete().Where(practicecatalogsignal.IDLTE(last.ID)).Exec(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// tryAdvisoryXactLock takes the advisory lock key for tx's transaction,
// reporting false when another transaction holds it.
func tryAdvisoryXactLock(ctx context.Context, tx *ent.Tx, key int64) (bool, error) {
	rows, err := tx.QueryContext(ctx, "SELECT pg_try_advisory_xact_lock($1)", key)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var locked bool
	if rows.Next() {
		if err := rows.Scan(&locked); err != nil {
			return false, err
		}
	}
	return locked, rows.Err()
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
	return domain.PracticeSubjectCatalog{Scope: row.Scope, FormatVersion: row.FormatVersion, BuiltAt: row.BuiltAt.UTC(), Items: items}, nil
}
