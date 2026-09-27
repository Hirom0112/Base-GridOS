package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Command struct {
	CommandID      string
	IdempotencyKey string
	DeviceID       string
	Generation     uint64
	SetpointKW     float64
	EffectiveAt    time.Time
	ExpiresAt      time.Time
}

type Acknowledgement struct {
	Accepted          bool
	NewPhysicalEffect bool
	RejectionReason   string
}

type BufferedObservation struct {
	ObservationID string
	Payload       []byte
}

type ObservationDraft struct {
	DeviceID string
	Build    func(uint64) (BufferedObservation, error)
}

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.initialize(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return store, nil
}

func (store *Store) initialize(ctx context.Context) error {
	_, err := store.db.ExecContext(ctx, `
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
CREATE TABLE IF NOT EXISTS commands (
  command_id TEXT PRIMARY KEY,
  idempotency_key TEXT NOT NULL UNIQUE,
  device_id TEXT NOT NULL,
  generation INTEGER NOT NULL,
  setpoint_kw REAL NOT NULL,
  effective_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS commands_device_generation ON commands(device_id, generation);
CREATE TABLE IF NOT EXISTS telemetry_buffer (
  observation_id TEXT PRIMARY KEY,
  payload BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS telemetry_sequences (
  device_id TEXT PRIMARY KEY,
  sequence INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS command_effects (
  command_id TEXT PRIMARY KEY REFERENCES commands(command_id),
  executed_at TEXT NOT NULL
);`)
	return err
}

func (store *Store) Close() error {
	return store.db.Close()
}

func (store *Store) AcceptCommand(ctx context.Context, command Command, now time.Time) (Acknowledgement, error) {
	if err := validateCommand(command); err != nil {
		return Acknowledgement{}, err
	}
	if !now.Before(command.ExpiresAt) {
		return Acknowledgement{RejectionReason: "EXPIRED"}, nil
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return Acknowledgement{}, err
	}
	finishWithoutCommit := func(acknowledgement Acknowledgement, operationErr error) (Acknowledgement, error) {
		return acknowledgement, errors.Join(operationErr, tx.Rollback())
	}
	existing, found, err := commandByID(ctx, tx, command.CommandID)
	if err != nil {
		return finishWithoutCommit(Acknowledgement{}, err)
	}
	if found {
		if existing == command {
			return finishWithoutCommit(Acknowledgement{Accepted: true}, nil)
		}
		return finishWithoutCommit(Acknowledgement{RejectionReason: "CONFLICTING_REUSE"}, nil)
	}
	latest, found, err := latestGeneration(ctx, tx, command.DeviceID)
	if err != nil {
		return finishWithoutCommit(Acknowledgement{}, err)
	}
	if found && command.Generation < latest {
		return finishWithoutCommit(Acknowledgement{RejectionReason: "OBSOLETE_GENERATION"}, nil)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO commands (
command_id, idempotency_key, device_id, generation, setpoint_kw, effective_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`, command.CommandID, command.IdempotencyKey, command.DeviceID, command.Generation, command.SetpointKW, formatTime(command.EffectiveAt), formatTime(command.ExpiresAt))
	if err != nil {
		return finishWithoutCommit(Acknowledgement{}, err)
	}
	if err := tx.Commit(); err != nil {
		return Acknowledgement{}, err
	}
	return Acknowledgement{Accepted: true, NewPhysicalEffect: true}, nil
}

type commandQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func commandByID(ctx context.Context, query commandQuerier, commandID string) (Command, bool, error) {
	row := query.QueryRowContext(ctx, `SELECT command_id, idempotency_key, device_id, generation, setpoint_kw, effective_at, expires_at FROM commands WHERE command_id = ?`, commandID)
	command, err := scanCommand(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Command{}, false, nil
	}
	return command, err == nil, err
}

func latestGeneration(ctx context.Context, query commandQuerier, deviceID string) (uint64, bool, error) {
	var generation uint64
	err := query.QueryRowContext(ctx, `SELECT generation FROM commands WHERE device_id = ? ORDER BY generation DESC LIMIT 1`, deviceID).Scan(&generation)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return generation, err == nil, err
}

func (store *Store) Commands(ctx context.Context) ([]Command, error) {
	return queryCommands(ctx, store.db, `SELECT command_id, idempotency_key, device_id, generation, setpoint_kw, effective_at, expires_at FROM commands ORDER BY command_id`)
}

func (store *Store) ExecutableCommands(ctx context.Context, now time.Time) ([]Command, error) {
	return queryCommands(ctx, store.db, `SELECT command_id, idempotency_key, device_id, generation, setpoint_kw, effective_at, expires_at FROM commands WHERE effective_at <= ? AND expires_at > ? ORDER BY command_id`, formatTime(now), formatTime(now))
}

type commandRowsQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryCommands(ctx context.Context, querier commandRowsQuerier, query string, arguments ...any) (commands []Command, err error) {
	rows, err := querier.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()
	for rows.Next() {
		command, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}
	return commands, rows.Err()
}

func (store *Store) ClaimExecutableCommands(ctx context.Context, now time.Time) ([]Command, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	commands, err := queryCommands(ctx, tx, `SELECT command_id, idempotency_key, device_id, generation, setpoint_kw, effective_at, expires_at
FROM commands WHERE effective_at <= ? AND expires_at > ? AND NOT EXISTS (
  SELECT 1 FROM command_effects WHERE command_effects.command_id = commands.command_id
) ORDER BY command_id`, formatTime(now), formatTime(now))
	if err != nil {
		return nil, errors.Join(err, tx.Rollback())
	}
	for _, command := range commands {
		if _, err := tx.ExecContext(ctx, `INSERT INTO command_effects (command_id, executed_at) VALUES (?, ?)`, command.CommandID, formatTime(now)); err != nil {
			return nil, errors.Join(err, tx.Rollback())
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return commands, nil
}

type commandScanner interface {
	Scan(...any) error
}

func scanCommand(scanner commandScanner) (Command, error) {
	var command Command
	var effectiveAt string
	var expiresAt string
	err := scanner.Scan(&command.CommandID, &command.IdempotencyKey, &command.DeviceID, &command.Generation, &command.SetpointKW, &effectiveAt, &expiresAt)
	if err != nil {
		return Command{}, err
	}
	command.EffectiveAt, err = time.Parse(time.RFC3339Nano, effectiveAt)
	if err != nil {
		return Command{}, err
	}
	command.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
	return command, err
}

func validateCommand(command Command) error {
	if command.CommandID == "" || command.IdempotencyKey == "" || command.DeviceID == "" {
		return errors.New("command identifiers are required")
	}
	if command.EffectiveAt.IsZero() || command.ExpiresAt.IsZero() || !command.EffectiveAt.Before(command.ExpiresAt) {
		return errors.New("invalid command window")
	}
	return nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func (store *Store) BufferObservation(ctx context.Context, observationID string, payload []byte) error {
	if observationID == "" || len(payload) == 0 {
		return errors.New("observation identifier and payload are required")
	}
	_, err := store.db.ExecContext(ctx, `INSERT INTO telemetry_buffer (observation_id, payload) VALUES (?, ?) ON CONFLICT(observation_id) DO NOTHING`, observationID, payload)
	return err
}

func (store *Store) BufferObservationBatch(ctx context.Context, drafts []ObservationDraft) (err error) {
	if len(drafts) == 0 {
		return nil
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	for _, draft := range drafts {
		if draft.DeviceID == "" || draft.Build == nil {
			return errors.New("observation draft requires device and builder")
		}
		var sequence uint64
		err = tx.QueryRowContext(ctx, `INSERT INTO telemetry_sequences (device_id, sequence) VALUES (?, 1)
ON CONFLICT(device_id) DO UPDATE SET sequence = sequence + 1
RETURNING sequence`, draft.DeviceID).Scan(&sequence)
		if err != nil {
			return err
		}
		var observation BufferedObservation
		observation, err = draft.Build(sequence)
		if err != nil {
			return err
		}
		if observation.ObservationID == "" || len(observation.Payload) == 0 {
			return errors.New("observation identifier and payload are required")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO telemetry_buffer (observation_id, payload) VALUES (?, ?)`, observation.ObservationID, observation.Payload)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (store *Store) NextTelemetrySequence(ctx context.Context, deviceID string) (uint64, error) {
	if deviceID == "" {
		return 0, errors.New("device identifier is required")
	}
	var sequence uint64
	err := store.db.QueryRowContext(ctx, `INSERT INTO telemetry_sequences (device_id, sequence) VALUES (?, 1)
ON CONFLICT(device_id) DO UPDATE SET sequence = sequence + 1
RETURNING sequence`, deviceID).Scan(&sequence)
	return sequence, err
}

func (store *Store) BufferedObservations(ctx context.Context) (observations []BufferedObservation, err error) {
	rows, err := store.db.QueryContext(ctx, `SELECT observation_id, payload FROM telemetry_buffer ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()
	for rows.Next() {
		var observation BufferedObservation
		if err := rows.Scan(&observation.ObservationID, &observation.Payload); err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	return observations, rows.Err()
}

func (store *Store) ConfirmObservation(ctx context.Context, observationID string) error {
	result, err := store.db.ExecContext(ctx, `DELETE FROM telemetry_buffer WHERE observation_id = ?`, observationID)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted != 1 {
		return fmt.Errorf("observation %q is not buffered", observationID)
	}
	return nil
}

func (store *Store) ConfirmObservations(ctx context.Context, observationIDs []string) (err error) {
	if len(observationIDs) == 0 {
		return nil
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	for _, observationID := range observationIDs {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `DELETE FROM telemetry_buffer WHERE observation_id = ?`, observationID)
		if err != nil {
			return err
		}
		var deleted int64
		deleted, err = result.RowsAffected()
		if err != nil {
			return err
		}
		if deleted != 1 {
			return fmt.Errorf("observation %q is not buffered", observationID)
		}
	}
	return tx.Commit()
}
