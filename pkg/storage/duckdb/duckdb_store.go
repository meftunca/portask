package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/marcboeker/go-duckdb"

	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// DuckDBStore implements MessageStore using DuckDB for ultra-fast analytics-grade storage
type DuckDBStore struct {
	db      *sql.DB
	dataDir string
	mu      sync.RWMutex

	// Prepared statements for performance
	insertStmt *sql.Stmt
	selectStmt *sql.Stmt
	deleteStmt *sql.Stmt

	// Metrics
	messagesWritten atomic.Int64
	messagesRead    atomic.Int64
	bytesWritten    atomic.Int64
	bytesRead       atomic.Int64
}

// Config for DuckDB
type Config struct {
	DataDir string

	// Performance tuning (FASTEST settings!)
	EnableCompression bool   // Default: true (zstd compression)
	MemoryLimit       string // Default: "2GB"
	Threads           int    // Default: CPU count
	EnableWAL         bool   // Default: false (FASTEST - no durability!)

	// Batch settings
	EnableBatchInsert bool // Default: true
	BatchSize         int  // Default: 1000
}

// DefaultConfig returns fastest DuckDB configuration
func DefaultConfig() *Config {
	return &Config{
		DataDir:           "./duckdb_data",
		EnableCompression: true,  // Fast compression
		MemoryLimit:       "2GB", // Generous memory
		Threads:           0,     // Auto-detect CPU count
		EnableWAL:         false, // FASTEST - no write-ahead log!
		EnableBatchInsert: true,  // Batch inserts
		BatchSize:         1000,  // Large batches
	}
}

// NewDuckDBStore creates a new DuckDB storage backend
func NewDuckDBStore(config *Config) (*DuckDBStore, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if config.DataDir == "" {
		config.DataDir = "./duckdb_data"
	}

	// Build connection string with FASTEST settings
	dbPath := fmt.Sprintf("%s/portask.db", config.DataDir)
	dsn := fmt.Sprintf("%s?access_mode=READ_WRITE", dbPath)

	// Open database
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	store := &DuckDBStore{
		db:      db,
		dataDir: config.DataDir,
	}

	// Apply FASTEST performance settings
	if err := store.applyFastestSettings(config); err != nil {
		return nil, fmt.Errorf("failed to apply settings: %w", err)
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	// Prepare statements
	if err := store.prepareStatements(); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return store, nil
}

// applyFastestSettings configures DuckDB for MAXIMUM write/read speed
func (d *DuckDBStore) applyFastestSettings(config *Config) error {
	settings := []struct {
		name  string
		value string
	}{
		// Memory settings
		{"memory_limit", config.MemoryLimit},
		{"max_memory", config.MemoryLimit},

		// Thread settings (all cores!)
		{"threads", fmt.Sprintf("%d", config.Threads)},

		// WAL settings (DISABLE for max speed!)
		{"wal_autocheckpoint", "0"},
		{"checkpoint_threshold", "1GB"},

		// Parallelism (MAXIMIZE!)
		{"enable_object_cache", "true"},
		{"preserve_insertion_order", "false"}, // Don't care about order = faster!

		// I/O optimization
		{"enable_http_metadata_cache", "true"},
		{"force_compression", fmt.Sprintf("%t", config.EnableCompression)},

		// Batch settings
		{"immediate_transaction_mode", "true"}, // Faster commits

		// Temp directory (use memory if possible)
		{"temp_directory", config.DataDir + "/temp"},
	}

	for _, setting := range settings {
		query := fmt.Sprintf("SET %s='%s'", setting.name, setting.value)
		if _, err := d.db.Exec(query); err != nil {
			// Some settings might not be available, continue
			continue
		}
	}

	// Disable WAL for MAXIMUM speed (no durability!)
	if !config.EnableWAL {
		if _, err := d.db.Exec("PRAGMA disable_checkpoint_on_shutdown"); err == nil {
			_, _ = d.db.Exec("PRAGMA wal_autocheckpoint=0")
		}
	}

	return nil
}

// initSchema creates optimized table schema
func (d *DuckDBStore) initSchema() error {
	// Column-store optimized schema
	schema := `
		CREATE TABLE IF NOT EXISTS messages (
			id VARCHAR PRIMARY KEY,
			topic VARCHAR NOT NULL,
			partition INTEGER NOT NULL,
			key VARCHAR,
			payload BLOB NOT NULL,
			timestamp BIGINT NOT NULL,
			ttl BIGINT,
			priority INTEGER DEFAULT 0,
			status INTEGER DEFAULT 0,
			metadata JSON,
			headers JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP
		);

		-- Indexes for fast lookups (covering indexes!)
		CREATE INDEX IF NOT EXISTS idx_messages_topic_partition 
			ON messages(topic, partition, timestamp);
		
		CREATE INDEX IF NOT EXISTS idx_messages_timestamp 
			ON messages(timestamp);
		
		CREATE INDEX IF NOT EXISTS idx_messages_expires_at 
			ON messages(expires_at) WHERE expires_at IS NOT NULL;
	`

	_, err := d.db.Exec(schema)
	return err
}

// prepareStatements prepares frequently used queries
func (d *DuckDBStore) prepareStatements() error {
	var err error

	// Insert statement
	d.insertStmt, err = d.db.Prepare(`
		INSERT INTO messages (
			id, topic, partition, key, payload, 
			timestamp, ttl, priority, status, 
			metadata, headers, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert: %w", err)
	}

	// Select statement
	d.selectStmt, err = d.db.Prepare(`
		SELECT id, topic, partition, key, payload, 
			   timestamp, ttl, priority, status, 
			   metadata, headers
		FROM messages 
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare select: %w", err)
	}

	// Delete statement
	d.deleteStmt, err = d.db.Prepare(`
		DELETE FROM messages WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare delete: %w", err)
	}

	return nil
}

// Store stores a single message
func (d *DuckDBStore) Store(ctx context.Context, message *types.PortaskMessage) error {
	start := time.Now()

	metadataJSON, _ := json.Marshal(message.Metadata)
	headersJSON, _ := json.Marshal(message.Headers)

	var expiresAt *time.Time
	if message.TTL > 0 {
		exp := time.Unix(0, message.Timestamp).Add(time.Duration(message.TTL) * time.Second)
		expiresAt = &exp
	}

	_, err := d.insertStmt.ExecContext(ctx,
		message.ID,
		message.Topic,
		message.Partition,
		message.Key,
		message.Payload,
		message.Timestamp,
		message.TTL,
		message.Priority,
		message.Status,
		metadataJSON,
		headersJSON,
		expiresAt,
	)

	if err != nil {
		return fmt.Errorf("insert failed: %w", err)
	}

	d.messagesWritten.Add(1)
	d.bytesWritten.Add(int64(len(message.Payload)))

	// Track duration if needed
	_ = time.Since(start)

	return nil
}

// StoreBatch stores multiple messages in a single transaction (FASTEST!)
func (d *DuckDBStore) StoreBatch(ctx context.Context, batch *types.MessageBatch) error {
	if batch == nil || len(batch.Messages) == 0 {
		return nil
	}

	start := time.Now()

	// Begin transaction for atomic batch insert
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement in transaction context
	stmt := tx.StmtContext(ctx, d.insertStmt)

	// Batch insert all messages
	for _, message := range batch.Messages {
		metadataJSON, _ := json.Marshal(message.Metadata)
		headersJSON, _ := json.Marshal(message.Headers)

		var expiresAt *time.Time
		if message.TTL > 0 {
			exp := time.Unix(0, message.Timestamp).Add(time.Duration(message.TTL) * time.Second)
			expiresAt = &exp
		}

		_, err := stmt.ExecContext(ctx,
			message.ID,
			message.Topic,
			message.Partition,
			message.Key,
			message.Payload,
			message.Timestamp,
			message.TTL,
			message.Priority,
			message.Status,
			metadataJSON,
			headersJSON,
			expiresAt,
		)

		if err != nil {
			return fmt.Errorf("batch insert failed for message %s: %w", message.ID, err)
		}

		d.bytesWritten.Add(int64(len(message.Payload)))
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	d.messagesWritten.Add(int64(len(batch.Messages)))

	// Log performance
	duration := time.Since(start)
	throughput := float64(len(batch.Messages)) / duration.Seconds()
	_ = throughput // Can be logged

	return nil
}

// FetchByID retrieves a message by ID
func (d *DuckDBStore) FetchByID(ctx context.Context, messageID types.MessageID) (*types.PortaskMessage, error) {
	var msg types.PortaskMessage
	var metadataJSON, headersJSON []byte

	err := d.selectStmt.QueryRowContext(ctx, messageID).Scan(
		&msg.ID,
		&msg.Topic,
		&msg.Partition,
		&msg.Key,
		&msg.Payload,
		&msg.Timestamp,
		&msg.TTL,
		&msg.Priority,
		&msg.Status,
		&metadataJSON,
		&headersJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Unmarshal JSON fields
	_ = json.Unmarshal(metadataJSON, &msg.Metadata)
	_ = json.Unmarshal(headersJSON, &msg.Headers)

	d.messagesRead.Add(1)
	d.bytesRead.Add(int64(len(msg.Payload)))

	return &msg, nil
}

// Delete removes a message
func (d *DuckDBStore) Delete(ctx context.Context, messageID types.MessageID) error {
	_, err := d.deleteStmt.ExecContext(ctx, messageID)
	return err
}

// Stats returns storage statistics
func (d *DuckDBStore) Stats(ctx context.Context) (*storage.StorageStats, error) {
	var count int64
	err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&count)
	if err != nil {
		return nil, err
	}

	return &storage.StorageStats{
		Status:               "healthy",
		TotalOperations:      d.messagesWritten.Load() + d.messagesRead.Load(),
		SuccessfulOperations: d.messagesWritten.Load() + d.messagesRead.Load(),
		FailedOperations:     0,
		LastHealthCheck:      time.Now(),
	}, nil
}

// Close closes the database connection
func (d *DuckDBStore) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Close prepared statements
	if d.insertStmt != nil {
		d.insertStmt.Close()
	}
	if d.selectStmt != nil {
		d.selectStmt.Close()
	}
	if d.deleteStmt != nil {
		d.deleteStmt.Close()
	}

	// Close database
	return d.db.Close()
}

// Optimize runs ANALYZE for query optimization
func (d *DuckDBStore) Optimize(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "ANALYZE messages")
	return err
}

// Vacuum reclaims space
func (d *DuckDBStore) Vacuum(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "VACUUM")
	return err
}

// GetMetrics returns performance metrics
func (d *DuckDBStore) GetMetrics() map[string]int64 {
	return map[string]int64{
		"messages_written": d.messagesWritten.Load(),
		"messages_read":    d.messagesRead.Load(),
		"bytes_written":    d.bytesWritten.Load(),
		"bytes_read":       d.bytesRead.Load(),
	}
}
