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
