//go:build !integration
// +build !integration

package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GIVEN a non existing build dir in the config
// WHEN the config is loaded
// THEN the directory should be created
func TestNewConfig(t *testing.T) {
	configDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(configDir, "yippee"), 0o755)
	assert.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", configDir)

	cacheDir := t.TempDir()

	config := map[string]string{"BuildDir": filepath.Join(cacheDir, "test-build-dir")}

	f, err := os.Create(filepath.Join(configDir, "yippee", "config.json"))
	assert.NoError(t, err)

	defer f.Close()

	configJSON, _ := json.Marshal(config)
	_, err = f.WriteString(string(configJSON))
	assert.NoError(t, err)

	newConfig, err := NewConfig(nil, GetConfigPath(), "v1.0.0")
	assert.NoError(t, err)

	assert.Equal(t, filepath.Join(cacheDir, "test-build-dir"), newConfig.BuildDir)

	_, err = os.Stat(filepath.Join(cacheDir, "test-build-dir"))
	assert.NoError(t, err)
}

// GIVEN a non existing build dir in the config and AURDEST set to a non-existing folder
// WHEN the config is loaded
// THEN the directory of AURDEST should be created and selected
func TestNewConfigAURDEST(t *testing.T) {
	configDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(configDir, "yippee"), 0o755)
	assert.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", configDir)

	cacheDir := t.TempDir()

	config := map[string]string{"BuildDir": filepath.Join(cacheDir, "test-other-dir")}
	t.Setenv("AURDEST", filepath.Join(cacheDir, "test-build-dir"))

	f, err := os.Create(filepath.Join(configDir, "yippee", "config.json"))
	assert.NoError(t, err)

	defer f.Close()

	configJSON, _ := json.Marshal(config)
	_, err = f.WriteString(string(configJSON))
	assert.NoError(t, err)

	newConfig, err := NewConfig(nil, GetConfigPath(), "v1.0.0")
	assert.NoError(t, err)

	assert.Equal(t, filepath.Join(cacheDir, "test-build-dir"), newConfig.BuildDir)

	_, err = os.Stat(filepath.Join(cacheDir, "test-build-dir"))
	assert.NoError(t, err)
}

// Test tilde expansion in AURDEST
func TestNewConfigAURDESTTildeExpansion(t *testing.T) {
	configDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(configDir, "yippee"), 0o755)
	assert.NoError(t, err)

	t.Setenv("XDG_CONFIG_HOME", configDir)

	homeDir := t.TempDir()
	cacheDir := t.TempDir()

	config := map[string]string{"BuildDir": filepath.Join(cacheDir, "test-other-dir")}
	t.Setenv("AURDEST", "~/test-build-dir")
	t.Setenv("HOME", homeDir)

	f, err := os.Create(filepath.Join(configDir, "yippee", "config.json"))
	assert.NoError(t, err)

	defer f.Close()

	configJSON, _ := json.Marshal(config)
	_, err = f.WriteString(string(configJSON))
	assert.NoError(t, err)

	newConfig, err := NewConfig(nil, GetConfigPath(), "v1.0.0")
	assert.NoError(t, err)

	assert.Equal(t, filepath.Join(homeDir, "test-build-dir"), newConfig.BuildDir)

	_, err = os.Stat(filepath.Join(homeDir, "test-build-dir"))
	assert.NoError(t, err)
}

// GIVEN default config
// WHEN setPrivilegeElevator gets called
// THEN sudobin should stay as "sudo" (given sudo exists)
func TestConfiguration_setPrivilegeElevator(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "sudo")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)

	assert.Equal(t, "sudo", config.SudoBin)
	assert.Equal(t, "-v", config.SudoFlags)
	assert.True(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN only su in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "su"
func TestConfiguration_setPrivilegeElevator_su(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "su")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()

	assert.NoError(t, err)
	assert.Equal(t, "su", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN no sudo in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "su"
func TestConfiguration_setPrivilegeElevator_no_path(t *testing.T) {
	t.Setenv("PATH", "")
	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	err := config.setPrivilegeElevator()

	assert.Error(t, err)
	assert.Equal(t, "sudo", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN doas in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "doas"
func TestConfiguration_setPrivilegeElevator_doas(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "doas")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)
	assert.Equal(t, "doas", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN config with wrapper and sudo loop enabled
// GIVEN wrapper is in path
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be kept as the wrapper
func TestConfiguration_setPrivilegeElevator_custom_script(t *testing.T) {
	path := t.TempDir()

	wrapper := filepath.Join(path, "custom-wrapper")
	_, err := os.Create(wrapper)
	os.Chmod(wrapper, 0o755)
	assert.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoLoop = true
	config.SudoBin = wrapper
	config.SudoFlags = "-v"

	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()

	assert.NoError(t, err)
	assert.Equal(t, wrapper, config.SudoBin)
	assert.Equal(t, "-v", config.SudoFlags)
	assert.True(t, config.SudoLoop)
}

// GIVEN default config and sudo loop enabled
// GIVEN doas as PACMAN_AUTH env variable
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "doas"
func TestConfiguration_setPrivilegeElevator_pacman_auth_doas(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "doas")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	require.NoError(t, err)

	sudo := filepath.Join(path, "sudo")
	_, err = os.Create(sudo)
	os.Chmod(sudo, 0o755)
	require.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoBin = "sudo"
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PACMAN_AUTH", "doas")
	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)
	assert.Equal(t, "doas", config.SudoBin)
	assert.Equal(t, "", config.SudoFlags)
	assert.False(t, config.SudoLoop)
}

// GIVEN config with doas configed and sudo loop enabled
// GIVEN sudo as PACMAN_AUTH env variable
// WHEN setPrivilegeElevator gets called
// THEN sudobin should be changed to "sudo"
func TestConfiguration_setPrivilegeElevator_pacman_auth_sudo(t *testing.T) {
	path := t.TempDir()

	doas := filepath.Join(path, "doas")
	_, err := os.Create(doas)
	os.Chmod(doas, 0o755)
	require.NoError(t, err)

	sudo := filepath.Join(path, "sudo")
	_, err = os.Create(sudo)
	os.Chmod(sudo, 0o755)
	require.NoError(t, err)

	config := DefaultConfig("test")
	config.SudoBin = "doas"
	config.SudoLoop = true
	config.SudoFlags = "-v"

	t.Setenv("PACMAN_AUTH", "sudo")
	t.Setenv("PATH", path)
	err = config.setPrivilegeElevator()
	assert.NoError(t, err)
	assert.Equal(t, "sudo", config.SudoBin)
	assert.Equal(t, "-v", config.SudoFlags)
	assert.True(t, config.SudoLoop)
}
