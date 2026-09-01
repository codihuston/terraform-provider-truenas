package services

import "context"

// SnapshotTaskServiceAPI defines the interface for periodic snapshot task operations.
type SnapshotTaskServiceAPI interface {
	Create(ctx context.Context, opts CreateSnapshotTaskOpts) (*SnapshotTask, error)
	Get(ctx context.Context, id int64) (*SnapshotTask, error)
	List(ctx context.Context) ([]SnapshotTask, error)
	Update(ctx context.Context, id int64, opts UpdateSnapshotTaskOpts) (*SnapshotTask, error)
	Delete(ctx context.Context, id int64) error
}

// Compile-time checks.
var _ SnapshotTaskServiceAPI = (*SnapshotTaskService)(nil)
var _ SnapshotTaskServiceAPI = (*MockSnapshotTaskService)(nil)

// MockSnapshotTaskService is a test double for SnapshotTaskServiceAPI.
type MockSnapshotTaskService struct {
	CreateFunc func(ctx context.Context, opts CreateSnapshotTaskOpts) (*SnapshotTask, error)
	GetFunc    func(ctx context.Context, id int64) (*SnapshotTask, error)
	ListFunc   func(ctx context.Context) ([]SnapshotTask, error)
	UpdateFunc func(ctx context.Context, id int64, opts UpdateSnapshotTaskOpts) (*SnapshotTask, error)
	DeleteFunc func(ctx context.Context, id int64) error
}

func (m *MockSnapshotTaskService) Create(ctx context.Context, opts CreateSnapshotTaskOpts) (*SnapshotTask, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, opts)
	}
	return nil, nil
}

func (m *MockSnapshotTaskService) Get(ctx context.Context, id int64) (*SnapshotTask, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockSnapshotTaskService) List(ctx context.Context) ([]SnapshotTask, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func (m *MockSnapshotTaskService) Update(ctx context.Context, id int64, opts UpdateSnapshotTaskOpts) (*SnapshotTask, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, opts)
	}
	return nil, nil
}

func (m *MockSnapshotTaskService) Delete(ctx context.Context, id int64) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
