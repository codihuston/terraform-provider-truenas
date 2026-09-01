package services

import "context"

// SystemServicesAPI defines the interface for system service operations.
type SystemServicesAPI interface {
	Get(ctx context.Context, name string) (*SystemService, error)
	List(ctx context.Context) ([]SystemService, error)
	SetEnable(ctx context.Context, name string, enable bool) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
}

// Compile-time checks.
var _ SystemServicesAPI = (*SystemServices)(nil)
var _ SystemServicesAPI = (*MockSystemServices)(nil)

// MockSystemServices is a test double for SystemServicesAPI.
type MockSystemServices struct {
	GetFunc       func(ctx context.Context, name string) (*SystemService, error)
	ListFunc      func(ctx context.Context) ([]SystemService, error)
	SetEnableFunc func(ctx context.Context, name string, enable bool) error
	StartFunc     func(ctx context.Context, name string) error
	StopFunc      func(ctx context.Context, name string) error
}

func (m *MockSystemServices) Get(ctx context.Context, name string) (*SystemService, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, name)
	}
	return nil, nil
}

func (m *MockSystemServices) List(ctx context.Context) ([]SystemService, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func (m *MockSystemServices) SetEnable(ctx context.Context, name string, enable bool) error {
	if m.SetEnableFunc != nil {
		return m.SetEnableFunc(ctx, name, enable)
	}
	return nil
}

func (m *MockSystemServices) Start(ctx context.Context, name string) error {
	if m.StartFunc != nil {
		return m.StartFunc(ctx, name)
	}
	return nil
}

func (m *MockSystemServices) Stop(ctx context.Context, name string) error {
	if m.StopFunc != nil {
		return m.StopFunc(ctx, name)
	}
	return nil
}
