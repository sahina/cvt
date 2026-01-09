package cvtservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLogger(t *testing.T) {
	t.Run("InitLogger Production", func(t *testing.T) {
		err := InitLogger(false) // production mode
		assert.NoError(t, err)
		assert.NotNil(t, GetLogger())

		Info("Test Info", zap.String("key", "value"))
		Debug("Test Debug") // Should not log in production default config usually, but method should work
		Warn("Test Warn")
		Error("Test Error")

		// Reset logger
		_ = Sync()
	})

	t.Run("InitLogger Development", func(t *testing.T) {
		err := InitLogger(true) // development mode
		assert.NoError(t, err)
		assert.NotNil(t, GetLogger())

		Info("Test Info Dev")
		Debug("Test Debug Dev")

		_ = Sync()
	})
}
