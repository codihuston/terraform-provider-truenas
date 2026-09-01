package services

import "context"

// GroupServiceAPI defines the interface for group operations.
type GroupServiceAPI interface {
	Create(ctx context.Context, opts CreateGroupOpts) (*Group, error)
	Get(ctx context.Context, id int64) (*Group, error)
	Update(ctx context.Context, id int64, opts UpdateGroupOpts) (*Group, error)
	Delete(ctx context.Context, id int64) error
	BuiltinUsersID(ctx context.Context) (int64, bool, error)
}

// Compile-time checks.
var _ GroupServiceAPI = (*GroupService)(nil)
var _ GroupServiceAPI = (*MockGroupService)(nil)

// MockGroupService is a test double for GroupServiceAPI.
type MockGroupService struct {
	CreateFunc         func(ctx context.Context, opts CreateGroupOpts) (*Group, error)
	GetFunc            func(ctx context.Context, id int64) (*Group, error)
	UpdateFunc         func(ctx context.Context, id int64, opts UpdateGroupOpts) (*Group, error)
	DeleteFunc         func(ctx context.Context, id int64) error
	BuiltinUsersIDFunc func(ctx context.Context) (int64, bool, error)
}

func (m *MockGroupService) Create(ctx context.Context, opts CreateGroupOpts) (*Group, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, opts)
	}
	return nil, nil
}

func (m *MockGroupService) Get(ctx context.Context, id int64) (*Group, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockGroupService) Update(ctx context.Context, id int64, opts UpdateGroupOpts) (*Group, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, opts)
	}
	return nil, nil
}

func (m *MockGroupService) Delete(ctx context.Context, id int64) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockGroupService) BuiltinUsersID(ctx context.Context) (int64, bool, error) {
	if m.BuiltinUsersIDFunc != nil {
		return m.BuiltinUsersIDFunc(ctx)
	}
	return 0, false, nil
}
