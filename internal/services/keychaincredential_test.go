package services

import (
	"context"
	"errors"
	"testing"

	"github.com/deevus/truenas-go/client"
)

// testSSHKeyPairJSON is the keychaincredential.create response the live API
// returns for an SSH_KEY_PAIR. The private key comes back too; the service
// drops it.
const testSSHKeyPairJSON = `{
	"id": 12,
	"name": "backup-host",
	"type": "SSH_KEY_PAIR",
	"attributes": {
		"private_key": "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret\n-----END OPENSSH PRIVATE KEY-----\n",
		"public_key": "ssh-ed25519 AAAAC3Nz backup-host\n"
	}
}`

// testSSHCredentialJSON is the keychaincredential.create response the live API
// returns for an SSH_CREDENTIALS.
const testSSHCredentialJSON = `{
	"id": 13,
	"name": "backup-host",
	"type": "SSH_CREDENTIALS",
	"attributes": {
		"host": "backup.example.com",
		"port": 2222,
		"username": "truenas_replication",
		"private_key": 12,
		"remote_host_key": "ssh-ed25519 AAAAC3Nz",
		"connect_timeout": 20
	}
}`

func testSSHCredentialOpts() CreateSSHCredentialOpts {
	return CreateSSHCredentialOpts{
		Name:           "backup-host",
		Host:           "backup.example.com",
		Port:           2222,
		Username:       "truenas_replication",
		PrivateKeyID:   12,
		RemoteHostKey:  "ssh-ed25519 AAAAC3Nz",
		ConnectTimeout: 20,
	}
}

func assertSSHKeyPair(t *testing.T, keypair *SSHKeyPair) {
	t.Helper()

	if keypair == nil {
		t.Fatal("expected key pair, got nil")
	}
	if keypair.ID != 12 {
		t.Errorf("expected ID 12, got %d", keypair.ID)
	}
	if keypair.Name != "backup-host" {
		t.Errorf("expected name 'backup-host', got %q", keypair.Name)
	}
	if keypair.PublicKey != "ssh-ed25519 AAAAC3Nz backup-host\n" {
		t.Errorf("unexpected public key: %q", keypair.PublicKey)
	}
}

func assertSSHCredential(t *testing.T, credential *SSHCredential) {
	t.Helper()

	if credential == nil {
		t.Fatal("expected credential, got nil")
	}
	if credential.ID != 13 {
		t.Errorf("expected ID 13, got %d", credential.ID)
	}
	if credential.Name != "backup-host" {
		t.Errorf("expected name 'backup-host', got %q", credential.Name)
	}
	if credential.Host != "backup.example.com" {
		t.Errorf("unexpected host: %q", credential.Host)
	}
	if credential.Port != 2222 {
		t.Errorf("expected port 2222, got %d", credential.Port)
	}
	if credential.Username != "truenas_replication" {
		t.Errorf("unexpected username: %q", credential.Username)
	}
	if credential.PrivateKeyID != 12 {
		t.Errorf("expected private key ID 12, got %d", credential.PrivateKeyID)
	}
	if credential.RemoteHostKey != "ssh-ed25519 AAAAC3Nz" {
		t.Errorf("unexpected remote host key: %q", credential.RemoteHostKey)
	}
	if credential.ConnectTimeout != 20 {
		t.Errorf("expected connect timeout 20, got %d", credential.ConnectTimeout)
	}
}

func TestKeychainCredentialService_CreateSSHKeyPair(t *testing.T) {
	c := &fakeCaller{result: testSSHKeyPairJSON}
	s := NewKeychainCredentialService(c)

	keypair, err := s.CreateSSHKeyPair(context.Background(), CreateSSHKeyPairOpts{
		Name:       "backup-host",
		PrivateKey: "PRIVATE",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "keychaincredential.create" {
		t.Errorf("expected method 'keychaincredential.create', got %q", c.method)
	}

	params, ok := c.params.(map[string]any)
	if !ok {
		t.Fatalf("expected map params, got %T", c.params)
	}
	if params["type"] != KeychainCredentialTypeSSHKeyPair {
		t.Errorf("unexpected type param: %v", params["type"])
	}
	if params["name"] != "backup-host" {
		t.Errorf("unexpected name param: %v", params["name"])
	}

	attrs, ok := params["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("expected map attributes, got %T", params["attributes"])
	}
	if attrs["private_key"] != "PRIVATE" {
		t.Errorf("unexpected private_key param: %v", attrs["private_key"])
	}
	// TrueNAS derives the public key, so submitting one would let the two
	// halves disagree.
	if _, ok := attrs["public_key"]; ok {
		t.Error("expected no public_key param")
	}

	assertSSHKeyPair(t, keypair)
}

func TestKeychainCredentialService_CreateSSHKeyPair_CallError(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.CreateSSHKeyPair(context.Background(), CreateSSHKeyPairOpts{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_CreateSSHKeyPair_BadJSON(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{result: `not json`})

	if _, err := s.CreateSSHKeyPair(context.Background(), CreateSSHKeyPairOpts{}); err == nil {
		t.Fatal("expected error")
	}
}

// A response whose attributes do not match the declared type must be reported,
// not silently mapped to zero values.
func TestKeychainCredentialService_CreateSSHKeyPair_BadAttributes(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{
		result: `{"id": 12, "name": "backup-host", "type": "SSH_KEY_PAIR", "attributes": "not an object"}`,
	})

	if _, err := s.CreateSSHKeyPair(context.Background(), CreateSSHKeyPairOpts{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_GetSSHKeyPair(t *testing.T) {
	c := &fakeCaller{result: testSSHKeyPairJSON}
	s := NewKeychainCredentialService(c)

	keypair, err := s.GetSSHKeyPair(context.Background(), 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "keychaincredential.get_instance" {
		t.Errorf("expected method 'keychaincredential.get_instance', got %q", c.method)
	}
	if c.params != int64(12) {
		t.Errorf("expected params 12, got %v", c.params)
	}

	assertSSHKeyPair(t, keypair)
}

func TestKeychainCredentialService_GetSSHKeyPair_NotFound(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{
		err: &client.JSONRPCError{Message: "[ENOENT] Instance does not exist"},
	})

	keypair, err := s.GetSSHKeyPair(context.Background(), 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keypair != nil {
		t.Errorf("expected nil key pair, got %+v", keypair)
	}
}

func TestKeychainCredentialService_GetSSHKeyPair_CallError(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.GetSSHKeyPair(context.Background(), 12); err == nil {
		t.Fatal("expected error")
	}
}

// The keychain holds both credential types behind one set of endpoints, so an
// ID naming the other type must be reported rather than read as empty fields.
func TestKeychainCredentialService_GetSSHKeyPair_WrongType(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{result: testSSHCredentialJSON})

	_, err := s.GetSSHKeyPair(context.Background(), 13)
	if err == nil {
		t.Fatal("expected error")
	}
	if want := "keychain credential 13 is of type SSH_CREDENTIALS, not SSH_KEY_PAIR"; err.Error() != want {
		t.Errorf("expected %q, got %q", want, err.Error())
	}
}

func TestKeychainCredentialService_UpdateSSHKeyPair_RotatesKey(t *testing.T) {
	c := &fakeCaller{result: testSSHKeyPairJSON}
	s := NewKeychainCredentialService(c)

	privateKey := "ROTATED"
	keypair, err := s.UpdateSSHKeyPair(context.Background(), 12, UpdateSSHKeyPairOpts{
		Name:       "backup-host",
		PrivateKey: &privateKey,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "keychaincredential.update" {
		t.Errorf("expected method 'keychaincredential.update', got %q", c.method)
	}

	args, ok := c.params.([]any)
	if !ok || len(args) != 2 {
		t.Fatalf("expected two positional params, got %v", c.params)
	}
	if args[0] != int64(12) {
		t.Errorf("expected ID 12, got %v", args[0])
	}

	params, ok := args[1].(map[string]any)
	if !ok {
		t.Fatalf("expected map params, got %T", args[1])
	}
	attrs, ok := params["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("expected map attributes, got %T", params["attributes"])
	}
	if attrs["private_key"] != "ROTATED" {
		t.Errorf("unexpected private_key param: %v", attrs["private_key"])
	}

	assertSSHKeyPair(t, keypair)
}

// Omitting attributes leaves the stored key pair alone, which is the only way
// to rename a key whose private half the provider does not hold.
func TestKeychainCredentialService_UpdateSSHKeyPair_KeepsKey(t *testing.T) {
	c := &fakeCaller{result: testSSHKeyPairJSON}
	s := NewKeychainCredentialService(c)

	if _, err := s.UpdateSSHKeyPair(context.Background(), 12, UpdateSSHKeyPairOpts{Name: "renamed"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := c.params.([]any)
	params := args[1].(map[string]any)
	if params["name"] != "renamed" {
		t.Errorf("unexpected name param: %v", params["name"])
	}
	if _, ok := params["attributes"]; ok {
		t.Error("expected no attributes param")
	}
}

func TestKeychainCredentialService_UpdateSSHKeyPair_CallError(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.UpdateSSHKeyPair(context.Background(), 12, UpdateSSHKeyPairOpts{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_CreateSSHCredential(t *testing.T) {
	c := &fakeCaller{result: testSSHCredentialJSON}
	s := NewKeychainCredentialService(c)

	credential, err := s.CreateSSHCredential(context.Background(), testSSHCredentialOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "keychaincredential.create" {
		t.Errorf("expected method 'keychaincredential.create', got %q", c.method)
	}

	params, ok := c.params.(map[string]any)
	if !ok {
		t.Fatalf("expected map params, got %T", c.params)
	}
	if params["type"] != KeychainCredentialTypeSSHCredentials {
		t.Errorf("unexpected type param: %v", params["type"])
	}

	attrs, ok := params["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("expected map attributes, got %T", params["attributes"])
	}
	for key, want := range map[string]any{
		"host":            "backup.example.com",
		"port":            int64(2222),
		"username":        "truenas_replication",
		"private_key":     int64(12),
		"remote_host_key": "ssh-ed25519 AAAAC3Nz",
		"connect_timeout": int64(20),
	} {
		if attrs[key] != want {
			t.Errorf("expected %s %v, got %v", key, want, attrs[key])
		}
	}

	assertSSHCredential(t, credential)
}

func TestKeychainCredentialService_CreateSSHCredential_CallError(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.CreateSSHCredential(context.Background(), testSSHCredentialOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_CreateSSHCredential_BadJSON(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{result: `not json`})

	if _, err := s.CreateSSHCredential(context.Background(), testSSHCredentialOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_CreateSSHCredential_BadAttributes(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{
		result: `{"id": 13, "name": "backup-host", "type": "SSH_CREDENTIALS", "attributes": []}`,
	})

	if _, err := s.CreateSSHCredential(context.Background(), testSSHCredentialOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_GetSSHCredential(t *testing.T) {
	c := &fakeCaller{result: testSSHCredentialJSON}
	s := NewKeychainCredentialService(c)

	credential, err := s.GetSSHCredential(context.Background(), 13)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "keychaincredential.get_instance" {
		t.Errorf("expected method 'keychaincredential.get_instance', got %q", c.method)
	}

	assertSSHCredential(t, credential)
}

func TestKeychainCredentialService_GetSSHCredential_NotFound(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{
		err: &client.JSONRPCError{Message: "[ENOENT] Instance does not exist"},
	})

	credential, err := s.GetSSHCredential(context.Background(), 13)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if credential != nil {
		t.Errorf("expected nil credential, got %+v", credential)
	}
}

func TestKeychainCredentialService_GetSSHCredential_CallError(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.GetSSHCredential(context.Background(), 13); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_GetSSHCredential_WrongType(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{result: testSSHKeyPairJSON})

	_, err := s.GetSSHCredential(context.Background(), 12)
	if err == nil {
		t.Fatal("expected error")
	}
	if want := "keychain credential 12 is of type SSH_KEY_PAIR, not SSH_CREDENTIALS"; err.Error() != want {
		t.Errorf("expected %q, got %q", want, err.Error())
	}
}

// The API requires the full attributes value on update, so every field has to
// be resubmitted even when only one changed.
func TestKeychainCredentialService_UpdateSSHCredential(t *testing.T) {
	c := &fakeCaller{result: testSSHCredentialJSON}
	s := NewKeychainCredentialService(c)

	credential, err := s.UpdateSSHCredential(context.Background(), 13, testSSHCredentialOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "keychaincredential.update" {
		t.Errorf("expected method 'keychaincredential.update', got %q", c.method)
	}

	args, ok := c.params.([]any)
	if !ok || len(args) != 2 {
		t.Fatalf("expected two positional params, got %v", c.params)
	}
	if args[0] != int64(13) {
		t.Errorf("expected ID 13, got %v", args[0])
	}

	params := args[1].(map[string]any)
	attrs, ok := params["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("expected map attributes, got %T", params["attributes"])
	}
	for _, key := range []string{"host", "port", "username", "private_key", "remote_host_key", "connect_timeout"} {
		if _, ok := attrs[key]; !ok {
			t.Errorf("expected %s attribute", key)
		}
	}
	// type cannot be changed, so submitting it would be rejected.
	if _, ok := params["type"]; ok {
		t.Error("expected no type param")
	}

	assertSSHCredential(t, credential)
}

func TestKeychainCredentialService_UpdateSSHCredential_CallError(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.UpdateSSHCredential(context.Background(), 13, testSSHCredentialOpts()); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_Delete(t *testing.T) {
	c := &fakeCaller{result: `null`}
	s := NewKeychainCredentialService(c)

	if err := s.Delete(context.Background(), 13); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "keychaincredential.delete" {
		t.Errorf("expected method 'keychaincredential.delete', got %q", c.method)
	}
	// Passing cascade would delete or disable the dependants instead of
	// failing, which Terraform has no way to represent.
	if c.params != int64(13) {
		t.Errorf("expected params 13, got %v", c.params)
	}
}

func TestKeychainCredentialService_Delete_CallError(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{err: errors.New("credential is used")})

	if err := s.Delete(context.Background(), 13); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_ScanRemoteHostKey(t *testing.T) {
	c := &fakeCaller{result: `"ssh-ed25519 AAAAC3Nz\nssh-rsa AAAAB3Nz"`}
	s := NewKeychainCredentialService(c)

	hostKey, err := s.ScanRemoteHostKey(context.Background(), ScanRemoteHostKeyOpts{
		Host:           "backup.example.com",
		Port:           2222,
		ConnectTimeout: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.method != "keychaincredential.remote_ssh_host_key_scan" {
		t.Errorf("expected method 'keychaincredential.remote_ssh_host_key_scan', got %q", c.method)
	}

	params, ok := c.params.(map[string]any)
	if !ok {
		t.Fatalf("expected map params, got %T", c.params)
	}
	if params["host"] != "backup.example.com" || params["port"] != int64(2222) || params["connect_timeout"] != int64(20) {
		t.Errorf("unexpected params: %v", params)
	}

	if hostKey != "ssh-ed25519 AAAAC3Nz\nssh-rsa AAAAB3Nz" {
		t.Errorf("unexpected host key: %q", hostKey)
	}
}

func TestKeychainCredentialService_ScanRemoteHostKey_CallError(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{err: errors.New("connection refused")})

	if _, err := s.ScanRemoteHostKey(context.Background(), ScanRemoteHostKeyOpts{Host: "backup.example.com"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeychainCredentialService_ScanRemoteHostKey_BadJSON(t *testing.T) {
	s := NewKeychainCredentialService(&fakeCaller{result: `{}`})

	if _, err := s.ScanRemoteHostKey(context.Background(), ScanRemoteHostKeyOpts{Host: "backup.example.com"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestMockKeychainCredentialService_Defaults(t *testing.T) {
	var m KeychainCredentialServiceAPI = &MockKeychainCredentialService{}
	ctx := context.Background()

	if keypair, err := m.CreateSSHKeyPair(ctx, CreateSSHKeyPairOpts{}); keypair != nil || err != nil {
		t.Error("expected nil, nil from default CreateSSHKeyPair")
	}
	if keypair, err := m.GetSSHKeyPair(ctx, 1); keypair != nil || err != nil {
		t.Error("expected nil, nil from default GetSSHKeyPair")
	}
	if keypair, err := m.UpdateSSHKeyPair(ctx, 1, UpdateSSHKeyPairOpts{}); keypair != nil || err != nil {
		t.Error("expected nil, nil from default UpdateSSHKeyPair")
	}
	if credential, err := m.CreateSSHCredential(ctx, CreateSSHCredentialOpts{}); credential != nil || err != nil {
		t.Error("expected nil, nil from default CreateSSHCredential")
	}
	if credential, err := m.GetSSHCredential(ctx, 1); credential != nil || err != nil {
		t.Error("expected nil, nil from default GetSSHCredential")
	}
	if credential, err := m.UpdateSSHCredential(ctx, 1, UpdateSSHCredentialOpts{}); credential != nil || err != nil {
		t.Error("expected nil, nil from default UpdateSSHCredential")
	}
	if err := m.Delete(ctx, 1); err != nil {
		t.Error("expected nil from default Delete")
	}
	if hostKey, err := m.ScanRemoteHostKey(ctx, ScanRemoteHostKeyOpts{}); hostKey != "" || err != nil {
		t.Error("expected empty, nil from default ScanRemoteHostKey")
	}
}

func TestMockKeychainCredentialService_Overrides(t *testing.T) {
	keypair := &SSHKeyPair{ID: 12}
	credential := &SSHCredential{ID: 13}
	m := &MockKeychainCredentialService{
		CreateSSHKeyPairFunc: func(ctx context.Context, opts CreateSSHKeyPairOpts) (*SSHKeyPair, error) {
			return keypair, nil
		},
		GetSSHKeyPairFunc: func(ctx context.Context, id int64) (*SSHKeyPair, error) { return keypair, nil },
		UpdateSSHKeyPairFunc: func(ctx context.Context, id int64, opts UpdateSSHKeyPairOpts) (*SSHKeyPair, error) {
			return keypair, nil
		},
		CreateSSHCredentialFunc: func(ctx context.Context, opts CreateSSHCredentialOpts) (*SSHCredential, error) {
			return credential, nil
		},
		GetSSHCredentialFunc: func(ctx context.Context, id int64) (*SSHCredential, error) { return credential, nil },
		UpdateSSHCredentialFunc: func(ctx context.Context, id int64, opts UpdateSSHCredentialOpts) (*SSHCredential, error) {
			return credential, nil
		},
		DeleteFunc:            func(ctx context.Context, id int64) error { return errors.New("boom") },
		ScanRemoteHostKeyFunc: func(ctx context.Context, opts ScanRemoteHostKeyOpts) (string, error) { return "key", nil },
	}
	ctx := context.Background()

	if got, _ := m.CreateSSHKeyPair(ctx, CreateSSHKeyPairOpts{}); got != keypair {
		t.Error("CreateSSHKeyPairFunc not used")
	}
	if got, _ := m.GetSSHKeyPair(ctx, 1); got != keypair {
		t.Error("GetSSHKeyPairFunc not used")
	}
	if got, _ := m.UpdateSSHKeyPair(ctx, 1, UpdateSSHKeyPairOpts{}); got != keypair {
		t.Error("UpdateSSHKeyPairFunc not used")
	}
	if got, _ := m.CreateSSHCredential(ctx, CreateSSHCredentialOpts{}); got != credential {
		t.Error("CreateSSHCredentialFunc not used")
	}
	if got, _ := m.GetSSHCredential(ctx, 1); got != credential {
		t.Error("GetSSHCredentialFunc not used")
	}
	if got, _ := m.UpdateSSHCredential(ctx, 1, UpdateSSHCredentialOpts{}); got != credential {
		t.Error("UpdateSSHCredentialFunc not used")
	}
	if err := m.Delete(ctx, 1); err == nil {
		t.Error("DeleteFunc not used")
	}
	if got, _ := m.ScanRemoteHostKey(ctx, ScanRemoteHostKeyOpts{}); got != "key" {
		t.Error("ScanRemoteHostKeyFunc not used")
	}
}
