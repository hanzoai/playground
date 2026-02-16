package server

import (
	"context"
	"fmt"

	"github.com/hanzoai/playground/control-plane/pkg/types"
)

type stubPackageStorage struct {
	packages map[string]*types.BotPackage
	getCalls []string
}

func newStubPackageStorage() *stubPackageStorage {
	return &stubPackageStorage{packages: make(map[string]*types.BotPackage)}
}

func (s *stubPackageStorage) GetBotPackage(ctx context.Context, packageID string) (*types.BotPackage, error) {
	s.getCalls = append(s.getCalls, packageID)
	if pkg, ok := s.packages[packageID]; ok {
		return pkg, nil
	}
	return nil, fmt.Errorf("package %s not found", packageID)
}

func (s *stubPackageStorage) StoreBotPackage(ctx context.Context, pkg *types.BotPackage) error {
	s.packages[pkg.ID] = pkg
	return nil
}
