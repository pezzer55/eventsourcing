package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hallgren/eventsourcing/core"
)

type SQL struct {
	db *sql.DB
}

// Open connection to database
func Open(db *sql.DB) *SQL {
	return &SQL{
		db: db,
	}
}

// Close the connection
func (s *SQL) Close() {
	s.db.Close()
}

// Save persists the snapshot
func (s *SQL) Save(snapshot core.Snapshot) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return errors.New(fmt.Sprintf("could not start a write transaction, %v", err))
	}
	defer tx.Rollback()

	statement := `SELECT id from snapshots where id=$1 AND type=$2 LIMIT 1`
	var id string
	err = tx.QueryRow(statement, snapshot.ID, snapshot.Type).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		// insert
		statement = `INSERT INTO snapshots (state, id, type, version, global_version) VALUES ($1, $2, $3, $4, $5)`
		_, err = tx.Exec(statement, string(snapshot.State), snapshot.ID, snapshot.Type, snapshot.Version, snapshot.GlobalVersion)
		if err != nil {
			return err
		}
	} else {
		// update
		statement = `UPDATE snapshots set state=$1, version=$2, global_version=$3 where id=$4 AND type=$5`
		_, err = tx.Exec(statement, string(snapshot.State), snapshot.Version, snapshot.GlobalVersion, snapshot.ID, snapshot.Type)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Get return the snapshot data from the database
func (s *SQL) Get(ctx context.Context, aggregateID, aggregateType string) (core.Snapshot, error) {
	var globalVersion core.Version
	var version core.Version
	var state []byte

	selectStm := `Select version, global_version, state from snapshots where id=$1 and type=$2`
	row := s.db.QueryRow(selectStm, aggregateID, aggregateType)
	if row.Err() != nil {
		return core.Snapshot{}, row.Err()
	}
	err := row.Scan(&version, &globalVersion, &state)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return core.Snapshot{}, core.ErrSnapshotNotFound
	} else if err != nil {
		return core.Snapshot{}, err
	}

	return core.Snapshot{
		ID:            aggregateID,
		Type:          aggregateType,
		Version:       version,
		GlobalVersion: globalVersion,
		State:         state,
	}, nil
}
