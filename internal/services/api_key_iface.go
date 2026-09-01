package services

import "context"

// APIKeyServiceAPI defines the interface for API key operations.
type APIKeyServiceAPI interface {
	Create(ctx context.Context, opts CreateAPIKeyOpts) (*APIKey, error)
	Get(ctx context.Context, id int64) (*APIKey, error)
	List(ctx context.Context) ([]APIKey, error)
	Update(ctx context.Context, id int64, opts UpdateAPIKeyOpts) (*APIKey, error)
	Delete(ctx context.Context, id int64) error
}

// Compile-time checks.
var _ APIKeyServiceAPI = (*APIKeyService)(nil)
var _ APIKeyServiceAPI = (*MockAPIKeyService)(nil)

// MockAPIKeyService is a test double for APIKeyServiceAPI.
type MockAPIKeyService struct {
	CreateFunc func(ctx context.Context, opts CreateAPIKeyOpts) (*APIKey, error)
	GetFunc    func(ctx context.Context, id int64) (*APIKey, error)
	ListFunc   func(ctx context.Context) ([]APIKey, error)
	UpdateFunc func(ctx context.Context, id int64, opts UpdateAPIKeyOpts) (*APIKey, error)
	DeleteFunc func(ctx context.Context, id int64) error
}

func (m *MockAPIKeyService) Create(ctx context.Context, opts CreateAPIKeyOpts) (*APIKey, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, opts)
	}
	return nil, nil
}

func (m *MockAPIKeyService) Get(ctx context.Context, id int64) (*APIKey, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockAPIKeyService) List(ctx context.Context) ([]APIKey, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func (m *MockAPIKeyService) Update(ctx context.Context, id int64, opts UpdateAPIKeyOpts) (*APIKey, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, opts)
	}
	return nil, nil
}

func (m *MockAPIKeyService) Delete(ctx context.Context, id int64) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
