package services

import "context"

// ReplicationServiceAPI defines the interface for replication task operations.
type ReplicationServiceAPI interface {
	Create(ctx context.Context, opts CreateReplicationTaskOpts) (*ReplicationTask, error)
	Get(ctx context.Context, id int64) (*ReplicationTask, error)
	List(ctx context.Context) ([]ReplicationTask, error)
	Update(ctx context.Context, id int64, opts UpdateReplicationTaskOpts) (*ReplicationTask, error)
	Delete(ctx context.Context, id int64) error
}

// Compile-time checks.
var _ ReplicationServiceAPI = (*ReplicationService)(nil)
var _ ReplicationServiceAPI = (*MockReplicationService)(nil)

// MockReplicationService is a test double for ReplicationServiceAPI.
type MockReplicationService struct {
	CreateFunc func(ctx context.Context, opts CreateReplicationTaskOpts) (*ReplicationTask, error)
	GetFunc    func(ctx context.Context, id int64) (*ReplicationTask, error)
	ListFunc   func(ctx context.Context) ([]ReplicationTask, error)
	UpdateFunc func(ctx context.Context, id int64, opts UpdateReplicationTaskOpts) (*ReplicationTask, error)
	DeleteFunc func(ctx context.Context, id int64) error
}

func (m *MockReplicationService) Create(ctx context.Context, opts CreateReplicationTaskOpts) (*ReplicationTask, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, opts)
	}
	return nil, nil
}

func (m *MockReplicationService) Get(ctx context.Context, id int64) (*ReplicationTask, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockReplicationService) List(ctx context.Context) ([]ReplicationTask, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func (m *MockReplicationService) Update(ctx context.Context, id int64, opts UpdateReplicationTaskOpts) (*ReplicationTask, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, opts)
	}
	return nil, nil
}

func (m *MockReplicationService) Delete(ctx context.Context, id int64) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
