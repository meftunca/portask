package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SchemaRegistry manages Avro and JSON schemas for Kafka messages
type SchemaRegistry struct {
	baseURL  string
	client   SchemaRegistryClient
	cache    map[string]*Schema
	idCache  map[int]*Schema
	subjects map[string]*Subject
	config   *RegistryConfig
	mutex    sync.RWMutex
}

// SchemaRegistryClient interface for HTTP operations
type SchemaRegistryClient interface {
	Get(ctx context.Context, path string) ([]byte, error)
	Post(ctx context.Context, path string, data []byte) ([]byte, error)
	Put(ctx context.Context, path string, data []byte) ([]byte, error)
	Delete(ctx context.Context, path string) ([]byte, error)
}

// Schema represents a schema in the registry
type Schema struct {
	ID         int                    `json:"id"`
	Subject    string                 `json:"subject"`
	Version    int                    `json:"version"`
	Schema     string                 `json:"schema"`
	Type       SchemaType             `json:"schemaType"`
	References []SchemaReference      `json:"references,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	RuleSet    *RuleSet               `json:"ruleSet,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
	UpdatedAt  time.Time              `json:"updatedAt"`
}

// SchemaType represents the type of schema
type SchemaType string

const (
	SchemaTypeAvro     SchemaType = "AVRO"
	SchemaTypeJSON     SchemaType = "JSON"
	SchemaTypeProtobuf SchemaType = "PROTOBUF"
)

// SchemaReference represents a reference to another schema
type SchemaReference struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Version int    `json:"version"`
}

// Subject represents a schema subject
type Subject struct {
	Name          string         `json:"name"`
	Schemas       []*Schema      `json:"schemas"`
	LatestVersion int            `json:"latestVersion"`
	Config        *SubjectConfig `json:"config"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// SubjectConfig represents configuration for a subject
type SubjectConfig struct {
	CompatibilityLevel CompatibilityLevel `json:"compatibilityLevel"`
	Normalize          bool               `json:"normalize"`
	ValidateFields     bool               `json:"validateFields"`
}

// CompatibilityLevel defines schema compatibility levels
type CompatibilityLevel string

const (
	CompatibilityNone               CompatibilityLevel = "NONE"
	CompatibilityBackward           CompatibilityLevel = "BACKWARD"
	CompatibilityBackwardTransitive CompatibilityLevel = "BACKWARD_TRANSITIVE"
	CompatibilityForward            CompatibilityLevel = "FORWARD"
	CompatibilityForwardTransitive  CompatibilityLevel = "FORWARD_TRANSITIVE"
	CompatibilityFull               CompatibilityLevel = "FULL"
	CompatibilityFullTransitive     CompatibilityLevel = "FULL_TRANSITIVE"
)

// RuleSet represents schema evolution rules
type RuleSet struct {
	DomainRules    []Rule `json:"domainRules"`
	MigrationRules []Rule `json:"migrationRules"`
}

// Rule represents a single schema rule
type Rule struct {
	Name      string                 `json:"name"`
	Doc       string                 `json:"doc"`
	Kind      RuleKind               `json:"kind"`
	Mode      RuleMode               `json:"mode"`
	Type      string                 `json:"type"`
	Tags      []string               `json:"tags"`
	Params    map[string]interface{} `json:"params"`
	Expr      string                 `json:"expr"`
	OnSuccess string                 `json:"onSuccess"`
	OnFailure string                 `json:"onFailure"`
	Disabled  bool                   `json:"disabled"`
}

// RuleKind represents the kind of rule
type RuleKind string

const (
	RuleKindTransform RuleKind = "TRANSFORM"
	RuleKindCondition RuleKind = "CONDITION"
)

// RuleMode represents when a rule is applied
type RuleMode string

const (
	RuleModeUpgrade   RuleMode = "UPGRADE"
	RuleModeDowngrade RuleMode = "DOWNGRADE"
	RuleModeRead      RuleMode = "READ"
	RuleModeWrite     RuleMode = "WRITE"
)

// RegistryConfig represents global registry configuration
type RegistryConfig struct {
	DefaultCompatibilityLevel CompatibilityLevel `json:"compatibilityLevel"`
	NormalizationEnabled      bool               `json:"normalize"`
	ValidateFields            bool               `json:"validateFields"`
	SchemasCacheSize          int                `json:"schemasCacheSize"`
}

// SchemaRegistryError represents an error from the schema registry
type SchemaRegistryError struct {
	ErrorCode int    `json:"error_code"`
	Message   string `json:"message"`
}

func (e SchemaRegistryError) Error() string {
	return fmt.Sprintf("schema registry error %d: %s", e.ErrorCode, e.Message)
}

// NewSchemaRegistry creates a new schema registry client
func NewSchemaRegistry(baseURL string, client SchemaRegistryClient) *SchemaRegistry {
	return &SchemaRegistry{
		baseURL:  baseURL,
		client:   client,
		cache:    make(map[string]*Schema),
		idCache:  make(map[int]*Schema),
		subjects: make(map[string]*Subject),
		config: &RegistryConfig{
			DefaultCompatibilityLevel: CompatibilityBackward,
			NormalizationEnabled:      true,
			ValidateFields:            true,
			SchemasCacheSize:          1000,
		},
	}
}

// RegisterSchema registers a new schema
func (sr *SchemaRegistry) RegisterSchema(ctx context.Context, subject string, schema *Schema) (int, error) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	// Validate schema
	if err := sr.validateSchema(schema); err != nil {
		return 0, fmt.Errorf("schema validation failed: %w", err)
	}

	// Check compatibility
	if err := sr.checkCompatibility(ctx, subject, schema); err != nil {
		return 0, fmt.Errorf("compatibility check failed: %w", err)
	}

	// Assign ID and version
	schema.ID = sr.generateSchemaID()
	schema.Subject = subject
	schema.CreatedAt = time.Now()
	schema.UpdatedAt = time.Now()

	// Get or create subject
	subj, exists := sr.subjects[subject]
	if !exists {
		subj = &Subject{
			Name:          subject,
			Schemas:       make([]*Schema, 0),
			LatestVersion: 0,
			Config: &SubjectConfig{
				CompatibilityLevel: sr.config.DefaultCompatibilityLevel,
				Normalize:          sr.config.NormalizationEnabled,
				ValidateFields:     sr.config.ValidateFields,
			},
			CreatedAt: time.Now(),
		}
		sr.subjects[subject] = subj
	}

	// Assign version
	schema.Version = subj.LatestVersion + 1
	subj.LatestVersion = schema.Version
	subj.UpdatedAt = time.Now()

	// Add to subject
	subj.Schemas = append(subj.Schemas, schema)

	// Cache schema
	cacheKey := fmt.Sprintf("%s:%d", subject, schema.Version)
	sr.cache[cacheKey] = schema
	sr.idCache[schema.ID] = schema

	return schema.ID, nil
}

// GetSchema retrieves a schema by subject and version
func (sr *SchemaRegistry) GetSchema(ctx context.Context, subject string, version int) (*Schema, error) {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%d", subject, version)
	if schema, exists := sr.cache[cacheKey]; exists {
		return schema, nil
	}

	// Get from subject
	subj, exists := sr.subjects[subject]
	if !exists {
		return nil, fmt.Errorf("subject not found: %s", subject)
	}

	for _, schema := range subj.Schemas {
		if schema.Version == version {
			sr.cache[cacheKey] = schema
			return schema, nil
		}
	}

	return nil, fmt.Errorf("schema not found for subject %s version %d", subject, version)
}

// GetSchemaByID retrieves a schema by its ID
func (sr *SchemaRegistry) GetSchemaByID(ctx context.Context, id int) (*Schema, error) {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	if schema, exists := sr.idCache[id]; exists {
		return schema, nil
	}

	return nil, fmt.Errorf("schema not found for ID %d", id)
}

// GetLatestSchema retrieves the latest version of a schema
func (sr *SchemaRegistry) GetLatestSchema(ctx context.Context, subject string) (*Schema, error) {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	subj, exists := sr.subjects[subject]
	if !exists {
		return nil, fmt.Errorf("subject not found: %s", subject)
	}

	if len(subj.Schemas) == 0 {
		return nil, fmt.Errorf("no schemas found for subject: %s", subject)
	}

	// Return latest schema
	return subj.Schemas[len(subj.Schemas)-1], nil
}

// GetSubjects returns all registered subjects
func (sr *SchemaRegistry) GetSubjects(ctx context.Context) ([]string, error) {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	subjects := make([]string, 0, len(sr.subjects))
	for name := range sr.subjects {
		subjects = append(subjects, name)
	}

	return subjects, nil
}

// GetVersions returns all versions for a subject
func (sr *SchemaRegistry) GetVersions(ctx context.Context, subject string) ([]int, error) {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	subj, exists := sr.subjects[subject]
	if !exists {
		return nil, fmt.Errorf("subject not found: %s", subject)
	}

	versions := make([]int, len(subj.Schemas))
	for i, schema := range subj.Schemas {
		versions[i] = schema.Version
	}

	return versions, nil
}

// DeleteSubject deletes a subject and all its schemas
func (sr *SchemaRegistry) DeleteSubject(ctx context.Context, subject string) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	subj, exists := sr.subjects[subject]
	if !exists {
		return fmt.Errorf("subject not found: %s", subject)
	}

	// Remove from caches
	for _, schema := range subj.Schemas {
		cacheKey := fmt.Sprintf("%s:%d", subject, schema.Version)
		delete(sr.cache, cacheKey)
		delete(sr.idCache, schema.ID)
	}

	// Remove subject
	delete(sr.subjects, subject)

	return nil
}

// DeleteSchemaVersion deletes a specific version of a schema
func (sr *SchemaRegistry) DeleteSchemaVersion(ctx context.Context, subject string, version int) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	subj, exists := sr.subjects[subject]
	if !exists {
		return fmt.Errorf("subject not found: %s", subject)
	}

	// Find and remove schema
	for i, schema := range subj.Schemas {
		if schema.Version == version {
			// Remove from caches
			cacheKey := fmt.Sprintf("%s:%d", subject, version)
			delete(sr.cache, cacheKey)
			delete(sr.idCache, schema.ID)

			// Remove from subject
			subj.Schemas = append(subj.Schemas[:i], subj.Schemas[i+1:]...)
			subj.UpdatedAt = time.Now()

			return nil
		}
	}

	return fmt.Errorf("schema version %d not found for subject %s", version, subject)
}

// CheckCompatibility checks if a schema is compatible with the subject
func (sr *SchemaRegistry) CheckCompatibility(ctx context.Context, subject string, schema *Schema) (bool, error) {
	return sr.checkCompatibility(ctx, subject, schema) == nil, nil
}

// UpdateSubjectConfig updates the configuration for a subject
func (sr *SchemaRegistry) UpdateSubjectConfig(ctx context.Context, subject string, config *SubjectConfig) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	subj, exists := sr.subjects[subject]
	if !exists {
		return fmt.Errorf("subject not found: %s", subject)
	}

	subj.Config = config
	subj.UpdatedAt = time.Now()

	return nil
}

// GetSubjectConfig returns the configuration for a subject
func (sr *SchemaRegistry) GetSubjectConfig(ctx context.Context, subject string) (*SubjectConfig, error) {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()

	subj, exists := sr.subjects[subject]
	if !exists {
		return nil, fmt.Errorf("subject not found: %s", subject)
	}

	return subj.Config, nil
}

// Internal helper methods

func (sr *SchemaRegistry) validateSchema(schema *Schema) error {
	if schema.Schema == "" {
		return fmt.Errorf("schema content cannot be empty")
	}

	switch schema.Type {
	case SchemaTypeAvro:
		return sr.validateAvroSchema(schema.Schema)
	case SchemaTypeJSON:
		return sr.validateJSONSchema(schema.Schema)
	case SchemaTypeProtobuf:
		return sr.validateProtobufSchema(schema.Schema)
	default:
		return fmt.Errorf("unsupported schema type: %s", schema.Type)
	}
}

func (sr *SchemaRegistry) validateAvroSchema(schemaStr string) error {
	// Basic JSON validation for Avro schema
	var avroSchema interface{}
	return json.Unmarshal([]byte(schemaStr), &avroSchema)
}

func (sr *SchemaRegistry) validateJSONSchema(schemaStr string) error {
	// Basic JSON validation for JSON schema
	var jsonSchema interface{}
	return json.Unmarshal([]byte(schemaStr), &jsonSchema)
}

func (sr *SchemaRegistry) validateProtobufSchema(schemaStr string) error {
	// Basic validation for Protobuf schema
	if len(schemaStr) == 0 {
		return fmt.Errorf("protobuf schema cannot be empty")
	}
	return nil
}

func (sr *SchemaRegistry) checkCompatibility(ctx context.Context, subject string, newSchema *Schema) error {
	subj, exists := sr.subjects[subject]
	if !exists {
		return nil // No existing schemas, always compatible
	}

	if len(subj.Schemas) == 0 {
		return nil // No existing schemas, always compatible
	}

	// Get the latest schema for compatibility check
	latestSchema := subj.Schemas[len(subj.Schemas)-1]

	return sr.isCompatible(latestSchema, newSchema, subj.Config.CompatibilityLevel)
}

func (sr *SchemaRegistry) isCompatible(existing, new *Schema, level CompatibilityLevel) error {
	switch level {
	case CompatibilityNone:
		return nil
	case CompatibilityBackward:
		return sr.checkBackwardCompatibility(existing, new)
	case CompatibilityForward:
		return sr.checkForwardCompatibility(existing, new)
	case CompatibilityFull:
		if err := sr.checkBackwardCompatibility(existing, new); err != nil {
			return err
		}
		return sr.checkForwardCompatibility(existing, new)
	default:
		return fmt.Errorf("unsupported compatibility level: %s", level)
	}
}

func (sr *SchemaRegistry) checkBackwardCompatibility(existing, new *Schema) error {
	// Simplified compatibility check
	// In a real implementation, this would do proper schema compatibility checking
	if existing.Type != new.Type {
		return fmt.Errorf("schema type mismatch: %s != %s", existing.Type, new.Type)
	}
	return nil
}

func (sr *SchemaRegistry) checkForwardCompatibility(existing, new *Schema) error {
	// Simplified compatibility check
	// In a real implementation, this would do proper schema compatibility checking
	if existing.Type != new.Type {
		return fmt.Errorf("schema type mismatch: %s != %s", existing.Type, new.Type)
	}
	return nil
}

func (sr *SchemaRegistry) generateSchemaID() int {
	// Simple ID generation - in production this would be more sophisticated
	return int(time.Now().UnixNano() % 1000000)
}

// ClearCache clears the schema cache
func (sr *SchemaRegistry) ClearCache() {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	sr.cache = make(map[string]*Schema)
	sr.idCache = make(map[int]*Schema)
}
