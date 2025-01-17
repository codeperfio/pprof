package driver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
)

// settings holds pprof settings.
type settings struct {
	// Configs holds a list of named UI configurations.
	Configs []namedConfig `json:"configs"`
}

// namedConfig associates a name with a PprofConfig.
type namedConfig struct {
	Name string `json:"name"`
	PprofConfig
}

// settingsFileName returns the name of the file where settings should be saved.
func settingsFileName() (string, error) {
	// Return "pprof/settings.json" under os.UserConfigDir().
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pprof", "settings.json"), nil
}

// readSettings reads settings from fname.
func readSettings(fname string) (*settings, error) {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return &settings{}, nil
		}
		return nil, fmt.Errorf("could not read settings: %w", err)
	}
	settings := &settings{}
	if err := json.Unmarshal(data, settings); err != nil {
		return nil, fmt.Errorf("could not parse settings: %w", err)
	}
	for i := range settings.Configs {
		settings.Configs[i].resetTransient()
	}
	return settings, nil
}

// writeSettings saves settings to fname.
func writeSettings(fname string, settings *settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode settings: %w", err)
	}

	// create the settings directory if it does not exist
	// XDG specifies permissions 0700 when creating settings dirs:
	// https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
	if err := os.MkdirAll(filepath.Dir(fname), 0700); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	if err := ioutil.WriteFile(fname, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}
	return nil
}

// configMenuEntry holds information for a single PprofConfig menu entry.
type configMenuEntry struct {
	Name       string
	URL        string
	Current    bool // Is this the currently selected PprofConfig?
	UserConfig bool // Is this a user-provided PprofConfig?
}

// configMenu returns a list of items to add to a menu in the web UI.
func configMenu(fname string, url url.URL) []configMenuEntry {
	// Start with system configs.
	configs := []namedConfig{{Name: "Default", PprofConfig: DefaultConfig()}}
	if settings, err := readSettings(fname); err == nil {
		// Add user configs.
		configs = append(configs, settings.Configs...)
	}

	// Convert to menu entries.
	result := make([]configMenuEntry, len(configs))
	lastMatch := -1
	for i, cfg := range configs {
		dst, changed := cfg.PprofConfig.makeURL(url)
		if !changed {
			lastMatch = i
		}
		result[i] = configMenuEntry{
			Name:       cfg.Name,
			URL:        dst.String(),
			UserConfig: (i != 0),
		}
	}
	// Mark the last matching PprofConfig as currennt
	if lastMatch >= 0 {
		result[lastMatch].Current = true
	}
	return result
}

// editSettings edits settings by applying fn to them.
func editSettings(fname string, fn func(s *settings) error) error {
	settings, err := readSettings(fname)
	if err != nil {
		return err
	}
	if err := fn(settings); err != nil {
		return err
	}
	return writeSettings(fname, settings)
}

// setConfig saves the PprofConfig specified in request to fname.
func setConfig(fname string, request url.URL) error {
	q := request.Query()
	name := q.Get("PprofConfig")
	if name == "" {
		return fmt.Errorf("invalid PprofConfig name")
	}
	cfg := currentConfig()
	if err := cfg.applyURL(q); err != nil {
		return err
	}
	return editSettings(fname, func(s *settings) error {
		for i, c := range s.Configs {
			if c.Name == name {
				s.Configs[i].PprofConfig = cfg
				return nil
			}
		}
		s.Configs = append(s.Configs, namedConfig{Name: name, PprofConfig: cfg})
		return nil
	})
}

// removeConfig removes PprofConfig from fname.
func removeConfig(fname, config string) error {
	return editSettings(fname, func(s *settings) error {
		for i, c := range s.Configs {
			if c.Name == config {
				s.Configs = append(s.Configs[:i], s.Configs[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("PprofConfig %s not found", config)
	})
}
