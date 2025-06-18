package watcher

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketProcessed = "processed"
	bucketFailed    = "failed"
)

// processingHistory implements ProcessingHistory interface using BoltDB
type processingHistory struct {
	db *bolt.DB
}

// NewProcessingHistory creates a new processing history with BoltDB
func NewProcessingHistory(dbPath string) (ProcessingHistory, error) {
	db, err := bolt.Open(dbPath, 0o600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open history database: %w", err)
	}

	// Create buckets if they don't exist
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketProcessed)); err != nil {
			return fmt.Errorf("failed to create processed bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bucketFailed)); err != nil {
			return fmt.Errorf("failed to create failed bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &processingHistory{db: db}, nil
}

// IsProcessed checks if a file hash has been processed
func (ph *processingHistory) IsProcessed(fileHash string) (bool, error) {
	var exists bool
	err := ph.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketProcessed))
		if bucket == nil {
			return nil
		}
		value := bucket.Get([]byte(fileHash))
		exists = value != nil
		return nil
	})
	return exists, err
}

// RecordProcessed records a successfully processed file
func (ph *processingHistory) RecordProcessed(fileHash string, info *ProcessedInfo) error {
	return ph.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketProcessed))
		if bucket == nil {
			return fmt.Errorf("processed bucket not found")
		}

		data, err := json.Marshal(info)
		if err != nil {
			return fmt.Errorf("failed to marshal processed info: %w", err)
		}

		if err := bucket.Put([]byte(fileHash), data); err != nil {
			return fmt.Errorf("failed to store processed info: %w", err)
		}

		// Remove from failed bucket if exists
		failedBucket := tx.Bucket([]byte(bucketFailed))
		if failedBucket != nil {
			_ = failedBucket.Delete([]byte(fileHash))
		}

		return nil
	})
}

// RecordFailed records a failed processing attempt
func (ph *processingHistory) RecordFailed(fileHash string, info *FailedInfo) error {
	return ph.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketFailed))
		if bucket == nil {
			return fmt.Errorf("failed bucket not found")
		}

		// Check if already failed, increment retry count
		existingData := bucket.Get([]byte(fileHash))
		if existingData != nil {
			var existing FailedInfo
			if err := json.Unmarshal(existingData, &existing); err == nil {
				info.RetryCount = existing.RetryCount + 1
			}
		}

		data, err := json.Marshal(info)
		if err != nil {
			return fmt.Errorf("failed to marshal failed info: %w", err)
		}

		if err := bucket.Put([]byte(fileHash), data); err != nil {
			return fmt.Errorf("failed to store failed info: %w", err)
		}

		return nil
	})
}

// GetProcessedInfo retrieves information about a processed file
func (ph *processingHistory) GetProcessedInfo(fileHash string) (*ProcessedInfo, error) {
	var info *ProcessedInfo
	err := ph.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketProcessed))
		if bucket == nil {
			return nil
		}

		data := bucket.Get([]byte(fileHash))
		if data == nil {
			return nil
		}

		var processedInfo ProcessedInfo
		if err := json.Unmarshal(data, &processedInfo); err != nil {
			return fmt.Errorf("failed to unmarshal processed info: %w", err)
		}

		info = &processedInfo
		return nil
	})
	return info, err
}

// GetFailedInfo retrieves information about a failed file
func (ph *processingHistory) GetFailedInfo(fileHash string) (*FailedInfo, error) {
	var info *FailedInfo
	err := ph.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketFailed))
		if bucket == nil {
			return nil
		}

		data := bucket.Get([]byte(fileHash))
		if data == nil {
			return nil
		}

		var failedInfo FailedInfo
		if err := json.Unmarshal(data, &failedInfo); err != nil {
			return fmt.Errorf("failed to unmarshal failed info: %w", err)
		}

		info = &failedInfo
		return nil
	})
	return info, err
}

// Close closes the underlying database
func (ph *processingHistory) Close() error {
	return ph.db.Close()
}
