package settings

import (
	"os"
	"path/filepath"
)

const (
	configFileName     string = "config.json" // configFileName holds the name of the config file.
	vcsFileName        string = "vcs.json"    // vcsFileName holds the name of the vcs file.
	completionFileName string = "completion.cache"
	systemdCache       string = "/var/cache/yippee" // systemd should handle cache creation
)

func GetConfigPath() string {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configDir := filepath.Join(configHome, "yippee")
		if err := initDir(configDir); err == nil {
			return filepath.Join(configDir, configFileName)
		}
	}

	if configHome := os.Getenv("HOME"); configHome != "" {
		configDir := filepath.Join(configHome, ".config", "yippee")
		if err := initDir(configDir); err == nil {
			return filepath.Join(configDir, configFileName)
		}
	}

	return ""
}

func getCacheHome() (string, error) {
	uid := os.Geteuid()

	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" && uid != 0 {
		cacheDir := filepath.Join(cacheHome, "yippee")
		if err := initDir(cacheDir); err == nil {
			return cacheDir, nil
		}
	}

	if cacheHome := os.Getenv("HOME"); cacheHome != "" && uid != 0 {
		cacheDir := filepath.Join(cacheHome, ".cache", "yippee")
		if err := initDir(cacheDir); err == nil {
			return cacheDir, nil
		}
	}

	if uid == 0 && os.Getenv("SUDO_USER") == "" && os.Getenv("DOAS_USER") == "" {
		return systemdCache, nil // Don't create directory if systemd-run takes care of it
	}

	tmpDir := filepath.Join(os.TempDir(), "yippee")

	return tmpDir, initDir(tmpDir)
}

func initDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return &ErrRuntimeDir{inner: err, dir: dir}
		}
	} else if err != nil {
		return err
	}

	return nil
}
