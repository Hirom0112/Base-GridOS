package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type LocalSink struct {
	directory string
	mu        sync.Mutex
}

func NewLocalSink(directory string) *LocalSink {
	return &LocalSink{directory: directory}
}

func (sink *LocalSink) Write(ctx context.Context, record Record) (writeErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.validate(); err != nil {
		return err
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if err := os.MkdirAll(sink.directory, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(sink.directory, "records.jsonl"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		writeErr = errors.Join(writeErr, file.Close())
	}()
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(file)
	for {
		var previous Record
		if err := decoder.Decode(&previous); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if previous.ID != record.ID {
			continue
		}
		prior, err := json.Marshal(previous)
		if err != nil {
			return err
		}
		if bytes.Equal(prior, encoded) {
			return nil
		}
		return errors.New("analytics record id reused with different content")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return file.Sync()
}
