package services

import "context"

// KeychainCredentialServiceAPI defines the interface for keychain credential
// operations.
type KeychainCredentialServiceAPI interface {
	CreateSSHKeyPair(ctx context.Context, opts CreateSSHKeyPairOpts) (*SSHKeyPair, error)
	GetSSHKeyPair(ctx context.Context, id int64) (*SSHKeyPair, error)
	UpdateSSHKeyPair(ctx context.Context, id int64, opts UpdateSSHKeyPairOpts) (*SSHKeyPair, error)
	CreateSSHCredential(ctx context.Context, opts CreateSSHCredentialOpts) (*SSHCredential, error)
	GetSSHCredential(ctx context.Context, id int64) (*SSHCredential, error)
	UpdateSSHCredential(ctx context.Context, id int64, opts UpdateSSHCredentialOpts) (*SSHCredential, error)
	Delete(ctx context.Context, id int64) error
	ScanRemoteHostKey(ctx context.Context, opts ScanRemoteHostKeyOpts) (string, error)
}

// Compile-time checks.
var _ KeychainCredentialServiceAPI = (*KeychainCredentialService)(nil)
var _ KeychainCredentialServiceAPI = (*MockKeychainCredentialService)(nil)

// MockKeychainCredentialService is a test double for
// KeychainCredentialServiceAPI.
type MockKeychainCredentialService struct {
	CreateSSHKeyPairFunc    func(ctx context.Context, opts CreateSSHKeyPairOpts) (*SSHKeyPair, error)
	GetSSHKeyPairFunc       func(ctx context.Context, id int64) (*SSHKeyPair, error)
	UpdateSSHKeyPairFunc    func(ctx context.Context, id int64, opts UpdateSSHKeyPairOpts) (*SSHKeyPair, error)
	CreateSSHCredentialFunc func(ctx context.Context, opts CreateSSHCredentialOpts) (*SSHCredential, error)
	GetSSHCredentialFunc    func(ctx context.Context, id int64) (*SSHCredential, error)
	UpdateSSHCredentialFunc func(ctx context.Context, id int64, opts UpdateSSHCredentialOpts) (*SSHCredential, error)
	DeleteFunc              func(ctx context.Context, id int64) error
	ScanRemoteHostKeyFunc   func(ctx context.Context, opts ScanRemoteHostKeyOpts) (string, error)
}

func (m *MockKeychainCredentialService) CreateSSHKeyPair(ctx context.Context, opts CreateSSHKeyPairOpts) (*SSHKeyPair, error) {
	if m.CreateSSHKeyPairFunc != nil {
		return m.CreateSSHKeyPairFunc(ctx, opts)
	}
	return nil, nil
}

func (m *MockKeychainCredentialService) GetSSHKeyPair(ctx context.Context, id int64) (*SSHKeyPair, error) {
	if m.GetSSHKeyPairFunc != nil {
		return m.GetSSHKeyPairFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockKeychainCredentialService) UpdateSSHKeyPair(ctx context.Context, id int64, opts UpdateSSHKeyPairOpts) (*SSHKeyPair, error) {
	if m.UpdateSSHKeyPairFunc != nil {
		return m.UpdateSSHKeyPairFunc(ctx, id, opts)
	}
	return nil, nil
}

func (m *MockKeychainCredentialService) CreateSSHCredential(ctx context.Context, opts CreateSSHCredentialOpts) (*SSHCredential, error) {
	if m.CreateSSHCredentialFunc != nil {
		return m.CreateSSHCredentialFunc(ctx, opts)
	}
	return nil, nil
}

func (m *MockKeychainCredentialService) GetSSHCredential(ctx context.Context, id int64) (*SSHCredential, error) {
	if m.GetSSHCredentialFunc != nil {
		return m.GetSSHCredentialFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockKeychainCredentialService) UpdateSSHCredential(ctx context.Context, id int64, opts UpdateSSHCredentialOpts) (*SSHCredential, error) {
	if m.UpdateSSHCredentialFunc != nil {
		return m.UpdateSSHCredentialFunc(ctx, id, opts)
	}
	return nil, nil
}

func (m *MockKeychainCredentialService) Delete(ctx context.Context, id int64) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockKeychainCredentialService) ScanRemoteHostKey(ctx context.Context, opts ScanRemoteHostKeyOpts) (string, error) {
	if m.ScanRemoteHostKeyFunc != nil {
		return m.ScanRemoteHostKeyFunc(ctx, opts)
	}
	return "", nil
}
