package pluginmgr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpandEnvTable covers the edge cases missed by the broader config
// tests: unterminated ${, interpolation in the middle of a value,
// multiple interpolations, empty-env-with-default, set-but-empty
// behavior, and default fallback when VAR is unset.
func TestExpandEnvTable(t *testing.T) {
	t.Setenv("FOO", "foo-value")
	t.Setenv("EMPTY_VAR", "")

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "literal_no_var", input: "hello", want: "hello"},
		{name: "single_var", input: "${FOO}", want: "foo-value"},
		{name: "var_embedded", input: "pre${FOO}post", want: "prefoo-valuepost"},
		{name: "multiple_vars", input: "${FOO}-${FOO}", want: "foo-value-foo-value"},
		{name: "default_used", input: "${MISSING:-fallback}", want: "fallback"},
		{name: "default_not_used", input: "${FOO:-fallback}", want: "foo-value"},
		{name: "empty_var_uses_default", input: "${EMPTY_VAR:-fallback}", want: "fallback",
			// Documents the current semantic: empty-string env var is
			// treated as unset. Users who want to pass an empty secret
			// must set the value explicitly in config, not via env.
		},
		{name: "unset_no_default", input: "${DEFINITELY_UNSET_DFKLSJF}", wantErr: "unset"},
		{name: "unterminated", input: "prefix ${FOO", wantErr: "unterminated"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandEnv(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestValidateBinaryPathEdges covers path-validation boundaries beyond
// the happy-path / obvious-escape cases: pluginRoot itself (rel="."),
// nested subdirectory (should succeed), and exact-parent boundary
// (rel="..").
func TestValidateBinaryPathEdges(t *testing.T) {
	t.Run("nested_subdir_allowed", func(t *testing.T) {
		tmp := t.TempDir()
		root := tmp + "/plugins"
		deeper := root + "/sub/cvt-plugin-foo"
		resolved, err := validateBinaryPath(deeper, root)
		require.NoError(t, err)
		assert.Equal(t, deeper, resolved)
	})

	t.Run("root_itself_rejected", func(t *testing.T) {
		tmp := t.TempDir()
		root := tmp + "/plugins"
		_, err := validateBinaryPath(root, root)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not the directory itself")
	})

	t.Run("parent_of_root_rejected", func(t *testing.T) {
		tmp := t.TempDir()
		root := tmp + "/plugins"
		parent := tmp
		_, err := validateBinaryPath(parent, root)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be under")
	})

	t.Run("sibling_of_root_rejected", func(t *testing.T) {
		tmp := t.TempDir()
		root := tmp + "/plugins"
		sibling := tmp + "/elsewhere/cvt-plugin-foo"
		_, err := validateBinaryPath(sibling, root)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be under")
	})

	t.Run("relative_path_rejected", func(t *testing.T) {
		_, err := validateBinaryPath("relative/path", "/abs/root")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "absolute")
	})
}
