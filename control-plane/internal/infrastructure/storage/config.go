// agents/internal/infrastructure/storage/config.go
package storage

import (
	"os"
	"path/filepath"

	"github.com/hanzoai/playground/control-plane/internal/core/domain"
	"github.com/hanzoai/playground/control-plane/internal/core/interfaces"
	"gopkg.in/yaml.v3"
)

type LocalConfigStorage struct {
	fs interfaces.FileSystemAdapter
}

func NewLocalConfigStorage(fs interfaces.FileSystemAdapter) interfaces.ConfigStorage {
	return &LocalConfigStorage{fs: fs}
}

func (s *LocalConfigStorage) LoadPlaygroundConfig(path string) (*domain.PlaygroundConfig, error) {
	if !s.fs.Exists(path) {
		return &domain.PlaygroundConfig{
			HomeDir:     filepath.Dir(path),
			Environment: make(map[string]string),
			MCP: domain.MCPConfig{
				Servers: []domain.MCPServer{},
			},
		}, nil
	}

	data, err := s.fs.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config domain.PlaygroundConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (s *LocalConfigStorage) SavePlaygroundConfig(path string, config *domain.PlaygroundConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return s.fs.WriteFile(path, data)
}
