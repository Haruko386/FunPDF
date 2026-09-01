package conf

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed models/*.json translators/*.json
var files embed.FS

func ReadTranslatorConfig(name string) ([]byte, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("invalid translator config name")
	}
	return files.ReadFile(path.Join("translators", name+".json"))
}

func ReadModelConfigs() ([][]byte, error) {
	entries, err := fs.ReadDir(files, "models")
	if err != nil {
		return nil, err
	}

	configs := make([][]byte, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := files.ReadFile(path.Join("models", entry.Name()))
		if err != nil {
			return nil, err
		}
		configs = append(configs, data)
	}

	return configs, nil
}
