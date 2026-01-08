package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore implements storage.Store using in-memory maps.
// Useful for testing and development.
type MemoryStore struct {
	mu          sync.RWMutex
	schemas     map[string]map[string]*SchemaRecord // schemaID -> version -> record
	validations []*ValidationRecord
	comparisons map[string]*ComparisonRecord // schemaID/old/new -> record
	consumers   map[string]*ConsumerRecord   // consumerID/environment -> record
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		schemas:     make(map[string]map[string]*SchemaRecord),
		validations: make([]*ValidationRecord, 0),
		comparisons: make(map[string]*ComparisonRecord),
		consumers:   make(map[string]*ConsumerRecord),
	}
}

func (s *MemoryStore) Migrate(ctx context.Context) error {
	return nil // No migrations needed for memory store
}

func (s *MemoryStore) Close() error {
	return nil
}

func (s *MemoryStore) Ping(ctx context.Context) error {
	return nil
}

func (s *MemoryStore) SetSchema(ctx context.Context, record *SchemaRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	if s.schemas[record.SchemaID] == nil {
		s.schemas[record.SchemaID] = make(map[string]*SchemaRecord)
	}

	// Mark all versions as not latest
	for _, r := range s.schemas[record.SchemaID] {
		r.IsLatest = false
	}

	record.IsLatest = true
	s.schemas[record.SchemaID][record.Version] = record

	return nil
}

func (s *MemoryStore) GetSchema(ctx context.Context, schemaID string) (*SchemaRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.schemas[schemaID]
	if !ok {
		return nil, &ErrNotFound{Resource: "schema", ID: schemaID}
	}

	for _, record := range versions {
		if record.IsLatest {
			return record, nil
		}
	}

	return nil, &ErrNotFound{Resource: "schema", ID: schemaID}
}

func (s *MemoryStore) GetSchemaVersion(ctx context.Context, schemaID, version string) (*SchemaRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.schemas[schemaID]
	if !ok {
		return nil, &ErrNotFound{Resource: "schema", ID: schemaID}
	}

	record, ok := versions[version]
	if !ok {
		return nil, &ErrNotFound{Resource: "schema version", ID: schemaID + "@" + version}
	}

	return record, nil
}

func (s *MemoryStore) DeleteSchema(ctx context.Context, schemaID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schemas[schemaID]; !ok {
		return &ErrNotFound{Resource: "schema", ID: schemaID}
	}

	delete(s.schemas, schemaID)
	return nil
}

func (s *MemoryStore) DeleteSchemaVersion(ctx context.Context, schemaID, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions, ok := s.schemas[schemaID]
	if !ok {
		return &ErrNotFound{Resource: "schema", ID: schemaID}
	}

	if _, ok := versions[version]; !ok {
		return &ErrNotFound{Resource: "schema version", ID: schemaID + "@" + version}
	}

	delete(versions, version)
	return nil
}

func (s *MemoryStore) ListSchemaIDs(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.schemas))
	for id := range s.schemas {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *MemoryStore) ListVersions(ctx context.Context, schemaID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.schemas[schemaID]
	if !ok {
		return nil, nil
	}

	result := make([]string, 0, len(versions))
	for v := range versions {
		result = append(result, v)
	}
	return result, nil
}

func (s *MemoryStore) ListSchemas(ctx context.Context, filter ListSchemasFilter) ([]*SchemaRecord, string, int32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SchemaRecord
	for _, versions := range s.schemas {
		for _, record := range versions {
			if !record.IsLatest {
				continue
			}
			if filter.Owner != "" && (record.Ownership == nil || record.Ownership.Owner != filter.Owner) {
				continue
			}
			if filter.Team != "" && (record.Ownership == nil || record.Ownership.Team != filter.Team) {
				continue
			}
			if filter.Environment != "" && record.Environment != filter.Environment {
				continue
			}
			result = append(result, record)
		}
	}

	return result, "", int32(len(result)), nil
}

func (s *MemoryStore) GetPreviousVersion(ctx context.Context, schemaID, currentVersion string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.schemas[schemaID]
	if !ok {
		return "", &ErrNotFound{Resource: "schema", ID: schemaID}
	}

	// Get current version's timestamp
	current, ok := versions[currentVersion]
	if !ok {
		return "", &ErrNotFound{Resource: "schema version", ID: schemaID + "@" + currentVersion}
	}

	var previousVersion string
	var previousTime time.Time

	for v, record := range versions {
		if v != currentVersion && record.RegisteredAt.Before(current.RegisteredAt) {
			if previousVersion == "" || record.RegisteredAt.After(previousTime) {
				previousVersion = v
				previousTime = record.RegisteredAt
			}
		}
	}

	if previousVersion == "" {
		return "", &ErrNotFound{Resource: "previous version", ID: schemaID}
	}

	return previousVersion, nil
}

func (s *MemoryStore) RecordValidation(ctx context.Context, record *ValidationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	s.validations = append(s.validations, record)
	return nil
}

func (s *MemoryStore) ListValidations(ctx context.Context, filter ListValidationsFilter) ([]*ValidationRecord, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ValidationRecord
	for _, record := range s.validations {
		if filter.SchemaID != "" && record.SchemaID != filter.SchemaID {
			continue
		}
		if filter.Method != "" && record.RequestMethod != filter.Method {
			continue
		}
		if filter.Environment != "" && record.Environment != filter.Environment {
			continue
		}
		if filter.Valid != nil && record.Valid != *filter.Valid {
			continue
		}
		if !filter.StartTime.IsZero() && record.ValidatedAt.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && record.ValidatedAt.After(filter.EndTime) {
			continue
		}
		result = append(result, record)
	}

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if len(result) > int(pageSize) {
		result = result[:pageSize]
	}

	return result, "", nil
}

func (s *MemoryStore) GetValidationAnalytics(ctx context.Context, filter ListValidationsFilter) (*ValidationAnalytics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	analytics := &ValidationAnalytics{
		ByMethod: make(map[string]int64),
		BySchema: make(map[string]int64),
	}

	for _, record := range s.validations {
		if filter.SchemaID != "" && record.SchemaID != filter.SchemaID {
			continue
		}
		if !filter.StartTime.IsZero() && record.ValidatedAt.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && record.ValidatedAt.After(filter.EndTime) {
			continue
		}

		analytics.TotalValidations++
		if record.Valid {
			analytics.PassCount++
		} else {
			analytics.FailCount++
		}
		analytics.ByMethod[record.RequestMethod]++
		analytics.BySchema[record.SchemaID]++
	}

	if analytics.TotalValidations > 0 {
		analytics.PassRate = float64(analytics.PassCount) / float64(analytics.TotalValidations) * 100
	}

	return analytics, nil
}

func (s *MemoryStore) RecordComparison(ctx context.Context, record *ComparisonRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	key := record.SchemaID + "/" + record.OldVersion + "/" + record.NewVersion
	s.comparisons[key] = record
	return nil
}

func (s *MemoryStore) GetComparison(ctx context.Context, schemaID, oldVersion, newVersion string) (*ComparisonRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := schemaID + "/" + oldVersion + "/" + newVersion
	record, ok := s.comparisons[key]
	if !ok {
		return nil, &ErrNotFound{Resource: "comparison", ID: key}
	}

	return record, nil
}

// consumerKey generates a unique key for a consumer registration.
func consumerKey(consumerID, schemaID, environment string) string {
	return consumerID + "/" + schemaID + "/" + environment
}

func (s *MemoryStore) RegisterConsumer(ctx context.Context, record *ConsumerRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	key := consumerKey(record.ConsumerID, record.SchemaID, record.Environment)

	// If consumer already exists, update it (upsert behavior)
	if existing, ok := s.consumers[key]; ok {
		record.RegisteredAt = existing.RegisteredAt // Preserve original registration time
	}

	s.consumers[key] = record
	return nil
}

func (s *MemoryStore) GetConsumer(ctx context.Context, consumerID, schemaID, environment string) (*ConsumerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := consumerKey(consumerID, schemaID, environment)
	record, ok := s.consumers[key]
	if !ok {
		return nil, &ErrNotFound{Resource: "consumer", ID: key}
	}

	return record, nil
}

func (s *MemoryStore) ListConsumers(ctx context.Context, filter ListConsumersFilter) ([]*ConsumerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ConsumerRecord
	for _, record := range s.consumers {
		if filter.SchemaID != "" && record.SchemaID != filter.SchemaID {
			continue
		}
		if filter.Environment != "" && record.Environment != filter.Environment {
			continue
		}
		if filter.ConsumerID != "" && record.ConsumerID != filter.ConsumerID {
			continue
		}
		result = append(result, record)
	}

	return result, nil
}

func (s *MemoryStore) DeregisterConsumer(ctx context.Context, consumerID, schemaID, environment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := consumerKey(consumerID, schemaID, environment)
	if _, ok := s.consumers[key]; !ok {
		return &ErrNotFound{Resource: "consumer", ID: key}
	}

	delete(s.consumers, key)
	return nil
}

func (s *MemoryStore) UpdateConsumerValidation(ctx context.Context, consumerID, schemaID, environment string, validatedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := consumerKey(consumerID, schemaID, environment)
	record, ok := s.consumers[key]
	if !ok {
		return &ErrNotFound{Resource: "consumer", ID: key}
	}

	record.LastValidatedAt = validatedAt
	return nil
}
