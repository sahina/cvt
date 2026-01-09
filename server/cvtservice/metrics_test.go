package cvtservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	assert.NotNil(t, m)
	assert.NotNil(t, m.SchemasRegistered)
	assert.NotNil(t, m.SchemaRegistrationErrors)
	assert.NotNil(t, m.ValidationsTotal)
	assert.NotNil(t, m.ValidationDuration)
	assert.NotNil(t, m.ValidationErrors)
	assert.NotNil(t, m.CacheHits)
	assert.NotNil(t, m.CacheMisses)
	assert.NotNil(t, m.CacheSize)
	assert.NotNil(t, m.CacheItemsCount)
	assert.NotNil(t, m.GrpcRequestsTotal)
	assert.NotNil(t, m.GrpcRequestDuration)
}
