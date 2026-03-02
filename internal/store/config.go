package store

import (
	"encoding/json"
	"os"

	"github.com/davidberget/cctask-go/internal/model"
)

func LoadConfig(projectRoot string) model.Config {
	fp := ConfigPath(projectRoot)
	data, err := os.ReadFile(fp)
	if err != nil {
		return model.Config{}
	}
	var c model.Config
	if err := json.Unmarshal(data, &c); err != nil {
		return model.Config{}
	}
	return c
}

func SaveConfig(projectRoot string, cfg model.Config) error {
	fp := ConfigPath(projectRoot)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fp, data, 0o644)
}
