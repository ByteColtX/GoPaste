// Package sqlite persists clipboard history in a local SQLite database.
//
// It is the SQLite adapter for history.Repository: the database file, the
// schema, migrations, and the encoding of an Item live here, while callers see
// only the repository interface. The driver is modernc.org/sqlite, which is
// pure Go, so the adapter builds and runs with CGO disabled.
//
// The database holds clipboard payloads, which are sensitive: it is a local
// file with no encryption and no network access. Where that file lives is the
// caller's decision, and callers should place it under the per-user
// application data directory rather than a shared or synced location.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gopaste/internal/history"

	sqlitedriver "modernc.org/sqlite"
)

// ErrEmptyPath is returned by Open when no database path was supplied.
var ErrEmptyPath = errors.New("history/sqlite: database path is empty")

// ErrInvalidPath is returned by Open when the path carries a query separator,
// which the adapter needs for its own connection settings.
var ErrInvalidPath = errors.New("history/sqlite: database path must not contain '?'")

// migrations holds one schema step per entry; migrations[n] takes a database
// from version n to version n+1. Steps are append-only: never edit a shipped
// step, add the next one.
var migrations = []string{
	`
CREATE TABLE history_items (
	id               TEXT    PRIMARY KEY,
	kind             INTEGER NOT NULL,
	content          TEXT    NOT NULL DEFAULT '',
	image_png        BLOB,
	image_width      INTEGER NOT NULL DEFAULT 0,
	image_height     INTEGER NOT NULL DEFAULT 0,
	file_size        INTEGER NOT NULL DEFAULT 0,
	source_device_id TEXT    NOT NULL DEFAULT '',
	created_at_ms    INTEGER NOT NULL,
	favorite         INTEGER NOT NULL DEFAULT 0,
	sync_state       INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX history_items_created_at ON history_items (created_at_ms DESC);

CREATE TABLE history_deliveries (
	item_id   TEXT    NOT NULL REFERENCES history_items (id) ON DELETE CASCADE,
	device_id TEXT    NOT NULL,
	state     INTEGER NOT NULL,
	position  INTEGER NOT NULL,
	PRIMARY KEY (item_id, device_id)
);
`,
	// v2 adds the rich-text markup. The capture path files a rich-text copy as
	// an item whose Text is the plain-text representation and whose HTML is the
	// markup beside it, so a schema without this column would keep the copy but
	// lose the formatting it was kept for.
	`
ALTER TABLE history_items ADD COLUMN html TEXT NOT NULL DEFAULT '';
`,
}

// Repository is a history.Repository backed by a local SQLite database.
type Repository struct {
	db   *sql.DB
	path string
}

var _ history.Repository = (*Repository)(nil)

// Open opens or creates the database at path and migrates it to the current
// schema. path is a filesystem path, or ":memory:" for a private in-memory
// database that is discarded on Close. An empty path returns ErrEmptyPath and a
// path containing "?" returns ErrInvalidPath.
//
// Open creates the file and applies the schema; it does not create the parent
// directory, does not seed any rows, and does not encrypt the file.
func Open(ctx context.Context, path string) (*Repository, error) {
	if strings.TrimSpace(path) == "" {
		return nil, ErrEmptyPath
	}
	if strings.ContainsRune(path, '?') {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
	connector, err := sqlitedriver.NewConnector(dataSourceName(path))
	if err != nil {
		return nil, fmt.Errorf("history/sqlite: open %s: %w", path, err)
	}
	db := sql.OpenDB(connector)
	// SQLite serializes writers anyway, and one pooled connection keeps a
	// ":memory:" database from being split across connections. History access
	// is short and infrequent, so the lost read concurrency costs nothing.
	db.SetMaxOpenConns(1)

	repository := &Repository{db: db, path: path}
	if err := repository.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repository, nil
}

// Load implements history.Repository.
func (r *Repository) Load(ctx context.Context) ([]history.Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, kind, content, html, image_png, image_width, image_height,
		        file_size, source_device_id, created_at_ms, favorite, sync_state
		 FROM history_items
		 ORDER BY created_at_ms DESC, rowid DESC`)
	if err != nil {
		return nil, fmt.Errorf("history/sqlite: load items: %w", err)
	}
	defer rows.Close()

	items := make([]history.Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history/sqlite: load items: %w", err)
	}
	if err := r.attachDeliveries(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

// Put implements history.Repository.
func (r *Repository) Put(ctx context.Context, items ...history.Item) error {
	if len(items) == 0 {
		return nil
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			return history.ErrInvalidItem
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("history/sqlite: put items: %w", err)
	}
	defer tx.Rollback()

	upsertItem, err := tx.PrepareContext(ctx, `
		INSERT INTO history_items (
			id, kind, content, html, image_png, image_width, image_height,
			file_size, source_device_id, created_at_ms, favorite, sync_state
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			kind             = excluded.kind,
			content          = excluded.content,
			html             = excluded.html,
			image_png        = excluded.image_png,
			image_width      = excluded.image_width,
			image_height     = excluded.image_height,
			file_size        = excluded.file_size,
			source_device_id = excluded.source_device_id,
			created_at_ms    = excluded.created_at_ms,
			favorite         = excluded.favorite,
			sync_state       = excluded.sync_state`)
	if err != nil {
		return fmt.Errorf("history/sqlite: put items: %w", err)
	}
	defer upsertItem.Close()

	clearDeliveries, err := tx.PrepareContext(ctx,
		`DELETE FROM history_deliveries WHERE item_id = ?`)
	if err != nil {
		return fmt.Errorf("history/sqlite: put items: %w", err)
	}
	defer clearDeliveries.Close()

	insertDelivery, err := tx.PrepareContext(ctx, `
		INSERT INTO history_deliveries (item_id, device_id, state, position)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (item_id, device_id) DO UPDATE SET
			state    = excluded.state,
			position = excluded.position`)
	if err != nil {
		return fmt.Errorf("history/sqlite: put items: %w", err)
	}
	defer insertDelivery.Close()

	for _, item := range items {
		var png []byte
		var width, height int
		if item.Image != nil && len(item.Image.PNG) > 0 {
			png = item.Image.PNG
			width = item.Image.Width
			height = item.Image.Height
		}
		if _, err := upsertItem.ExecContext(ctx,
			item.ID,
			int(item.Kind),
			item.Text,
			item.HTML,
			png,
			width,
			height,
			item.FileSize,
			item.SourceDeviceID,
			item.CreatedAt.UnixMilli(),
			boolToInt(item.Favorite),
			int(item.Sync),
		); err != nil {
			return fmt.Errorf("history/sqlite: put item %s: %w", item.ID, err)
		}

		if _, err := clearDeliveries.ExecContext(ctx, item.ID); err != nil {
			return fmt.Errorf("history/sqlite: put item %s: %w", item.ID, err)
		}
		for position, delivery := range item.Deliveries {
			if _, err := insertDelivery.ExecContext(ctx,
				item.ID,
				delivery.DeviceID,
				int(delivery.State),
				position,
			); err != nil {
				return fmt.Errorf("history/sqlite: put item %s: %w", item.ID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("history/sqlite: put items: %w", err)
	}
	return nil
}

// Delete implements history.Repository.
func (r *Repository) Delete(ctx context.Context, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("history/sqlite: delete items: %w", err)
	}
	defer tx.Rollback()

	// Delivery rows go with the item through ON DELETE CASCADE, which the
	// connection-level foreign_keys setting enables.
	statement, err := tx.PrepareContext(ctx, `DELETE FROM history_items WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("history/sqlite: delete items: %w", err)
	}
	defer statement.Close()

	for _, id := range ids {
		if _, err := statement.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("history/sqlite: delete item %s: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("history/sqlite: delete items: %w", err)
	}
	return nil
}

// Close implements history.Repository.
func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// attachDeliveries fills in each loaded item's delivery records, preserving the
// order they were stored in.
func (r *Repository) attachDeliveries(ctx context.Context, items []history.Item) error {
	if len(items) == 0 {
		return nil
	}
	index := make(map[string]int, len(items))
	for i := range items {
		index[items[i].ID] = i
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT item_id, device_id, state
		 FROM history_deliveries
		 ORDER BY item_id, position`)
	if err != nil {
		return fmt.Errorf("history/sqlite: load deliveries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			itemID   string
			deviceID string
			state    int
		)
		if err := rows.Scan(&itemID, &deviceID, &state); err != nil {
			return fmt.Errorf("history/sqlite: load deliveries: %w", err)
		}
		i, ok := index[itemID]
		if !ok {
			// A delivery whose item this load did not return; nothing to
			// attach it to.
			continue
		}
		items[i].Deliveries = append(items[i].Deliveries, history.DeviceDelivery{
			DeviceID: deviceID,
			State:    history.DeliveryState(state),
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("history/sqlite: load deliveries: %w", err)
	}
	return nil
}

func (r *Repository) migrate(ctx context.Context) error {
	version, err := r.schemaVersion(ctx)
	if err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf(
			"history/sqlite: %s: schema version %d is newer than this build supports (%d)",
			r.path, version, len(migrations))
	}
	for from := version; from < len(migrations); from++ {
		if err := r.applyMigration(ctx, from+1, migrations[from]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) schemaVersion(ctx context.Context) (int, error) {
	var version int
	if err := r.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("history/sqlite: %s: read schema version: %w", r.path, err)
	}
	return version, nil
}

// applyMigration runs one schema step and records its version in the same
// transaction, so a failed step leaves the database at its previous version.
func (r *Repository) applyMigration(ctx context.Context, version int, step string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("history/sqlite: %s: migrate to version %d: %w", r.path, version, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, step); err != nil {
		return fmt.Errorf("history/sqlite: %s: migrate to version %d: %w", r.path, version, err)
	}
	// PRAGMA takes no placeholders; version comes from the migrations table,
	// never from a caller.
	record := fmt.Sprintf("PRAGMA user_version = %d", version)
	if _, err := tx.ExecContext(ctx, record); err != nil {
		return fmt.Errorf("history/sqlite: %s: record schema version %d: %w", r.path, version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("history/sqlite: %s: migrate to version %d: %w", r.path, version, err)
	}
	return nil
}

// dataSourceName adds the connection settings the adapter depends on. The
// driver validates these shorthand keys before applying any of them, so a typo
// fails Open instead of silently dropping a setting.
func dataSourceName(path string) string {
	params := url.Values{
		"_journal_mode": {"WAL"},
		"_synchronous":  {"NORMAL"},
		"_busy_timeout": {"5000"},
		"_foreign_keys": {"1"},
	}
	return path + "?" + params.Encode()
}

func scanItem(rows *sql.Rows) (history.Item, error) {
	var (
		item      history.Item
		kind      int
		png       []byte
		width     int
		height    int
		createdMS int64
		favorite  int
		syncState int
	)
	if err := rows.Scan(
		&item.ID,
		&kind,
		&item.Text,
		&item.HTML,
		&png,
		&width,
		&height,
		&item.FileSize,
		&item.SourceDeviceID,
		&createdMS,
		&favorite,
		&syncState,
	); err != nil {
		return history.Item{}, fmt.Errorf("history/sqlite: read item row: %w", err)
	}
	item.Kind = history.Kind(kind)
	if png != nil {
		item.Image = &history.Image{PNG: png, Width: width, Height: height}
	}
	item.CreatedAt = time.UnixMilli(createdMS).UTC()
	item.Favorite = favorite != 0
	item.Sync = history.SyncState(syncState)
	return item, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
