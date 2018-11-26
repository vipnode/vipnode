package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger"
)

// MigrateLatest converts the database to the latest version that we know of.
func MigrateLatest(db *badger.DB, id string) error {
	m := Migration{
		Steps:         migrations[:],
		LatestVersion: dbVersion,
		DatabaseID:    id,
	}
	return m.Migrate(db)
}

// MigrationStep is called within an update transaction with the current version.
// It should update the database state to the next version. The migration step
// should update the database version with each step.
type MigrationStep func(txn *badger.Txn) error

// Migration handles transforming the database state to the newest version.
type Migration struct {
	// StartVersion is the minimum version that is supported by the migration.
	StartVersion int
	// LatestVersion is the final version that we should expect after running migrations.
	LatestVersion int
	// Steps are the migration steps for transforming the database state across versions when run in sequence.
	Steps []MigrationStep
	// DatabaseID is an identifier for the database being migrated (e.g. opts.Dir), used for more descriptive errors.
	DatabaseID string
}

func (m *Migration) error(cause error, version int) MigrationError {
	return MigrationError{
		OldVersion: version,
		NewVersion: m.LatestVersion,
		Path:       m.DatabaseID,
		Cause:      cause,
	}
}

// Migrate performs the migration sequence.
func (m *Migration) Migrate(db *badger.DB) error {
	// Attempt to migrate
	return db.Update(func(txn *badger.Txn) error {
		oldVersion, err := getVersion(txn)
		if err != nil {
			return m.error(err, oldVersion)
		}

		if oldVersion == m.LatestVersion {
			// No need to migrate
			return nil
		}

		if m.LatestVersion < oldVersion {
			return m.error(errors.New("database is newer than the supported version"), oldVersion)
		}

		if m.StartVersion > oldVersion {
			return m.error(errors.New("database version too old, migration is not supported"), oldVersion)
		}

		// Migration from oldVersion to m.LatestVersion
		for v := oldVersion; v < m.LatestVersion; {
			err := m.Steps[v-m.StartVersion](txn)
			if err != nil {
				return m.error(err, v)
			}
			nextVersion, err := getVersion(txn)
			if err != nil {
				return m.error(err, v)
			}
			if nextVersion <= v {
				err = errors.New("migration failed to increment version")
				return m.error(err, v)
			}
			v = nextVersion
		}

		return nil
	})
}

func checkVersion(txn *badger.Txn, assertVersion int) error {
	version, err := getVersion(txn)
	if err != nil {
		return err
	}
	if version != assertVersion {
		return errors.New("wrong version for migration")
	}
	return nil
}

func getVersion(txn *badger.Txn) (int, error) {
	versionKey := []byte("vip:version")
	var version int

	if err := getItem(txn, versionKey, &version); err != nil && err != badger.ErrKeyNotFound {
		return version, err
	}
	return version, nil
}

func setVersion(txn *badger.Txn, version int) error {
	versionKey := []byte("vip:version")
	return setItem(txn, versionKey, &version)
}

// MigrationError is returned when the database is opened with an outdated
// version and migation fails.
type MigrationError struct {
	OldVersion int
	NewVersion int
	Path       string
	Cause      error
}

func (err MigrationError) Error() string {
	return fmt.Sprintf("badger database migration error: Failed to migrate from version %d to %d at path %q: %s", err.OldVersion, err.NewVersion, err.Path, err.Cause)
}
