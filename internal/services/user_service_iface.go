package services

import "context"

// UserServiceAPI defines the interface for user account operations.
type UserServiceAPI interface {
	Create(ctx context.Context, opts CreateUserOpts) (*User, error)
	Get(ctx context.Context, id int64) (*User, error)
	Update(ctx context.Context, id int64, opts UpdateUserOpts) (*User, error)
	Delete(ctx context.Context, id int64, deleteGroup bool) error
}

// Compile-time checks.
var _ UserServiceAPI = (*UserService)(nil)
var _ UserServiceAPI = (*MockUserService)(nil)

// MockUserService is a test double for UserServiceAPI.
type MockUserService struct {
	CreateFunc func(ctx context.Context, opts CreateUserOpts) (*User, error)
	GetFunc    func(ctx context.Context, id int64) (*User, error)
	UpdateFunc func(ctx context.Context, id int64, opts UpdateUserOpts) (*User, error)
	DeleteFunc func(ctx context.Context, id int64, deleteGroup bool) error
}

func (m *MockUserService) Create(ctx context.Context, opts CreateUserOpts) (*User, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, opts)
	}
	return nil, nil
}

func (m *MockUserService) Get(ctx context.Context, id int64) (*User, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockUserService) Update(ctx context.Context, id int64, opts UpdateUserOpts) (*User, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, opts)
	}
	return nil, nil
}

func (m *MockUserService) Delete(ctx context.Context, id int64, deleteGroup bool) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id, deleteGroup)
	}
	return nil
}
