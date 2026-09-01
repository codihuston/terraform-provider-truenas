package services

import (
	"context"
	"encoding/json"
	"fmt"

	truenas "github.com/deevus/truenas-go"
)

// Keychain credential types. TrueNAS stores an SSH connection as two linked
// objects: the private key (SSH_KEY_PAIR) and the connection that uses it
// (SSH_CREDENTIALS).
const (
	KeychainCredentialTypeSSHKeyPair     = "SSH_KEY_PAIR"
	KeychainCredentialTypeSSHCredentials = "SSH_CREDENTIALS"
)

// SSHKeyPair is the user-facing representation of an SSH_KEY_PAIR keychain
// credential. The private key is deliberately absent: the provider writes it
// and never reads it back.
type SSHKeyPair struct {
	ID        int64
	Name      string
	PublicKey string
}

// CreateSSHKeyPairOpts contains options for creating an SSH key pair. TrueNAS
// derives the public key from the private key, so only the latter is sent.
type CreateSSHKeyPairOpts struct {
	Name       string
	PrivateKey string
}

// UpdateSSHKeyPairOpts contains options for updating an SSH key pair. A nil
// PrivateKey omits `attributes` from the request, which leaves the stored key
// pair untouched — the only way to rename a key whose private half the
// provider does not hold.
type UpdateSSHKeyPairOpts struct {
	Name       string
	PrivateKey *string
}

// SSHCredential is the user-facing representation of an SSH_CREDENTIALS
// keychain credential: an SSH connection to a remote host, authenticated with
// a key pair held in the same keychain.
type SSHCredential struct {
	ID             int64
	Name           string
	Host           string
	Port           int64
	Username       string
	PrivateKeyID   int64
	RemoteHostKey  string
	ConnectTimeout int64
}

// CreateSSHCredentialOpts contains options for creating an SSH connection.
// All fields are always sent on create.
type CreateSSHCredentialOpts struct {
	Name           string
	Host           string
	Port           int64
	Username       string
	PrivateKeyID   int64
	RemoteHostKey  string
	ConnectTimeout int64
}

// UpdateSSHCredentialOpts contains options for updating an SSH connection. The
// API requires the full `attributes` value on update, so all fields are always
// sent.
type UpdateSSHCredentialOpts = CreateSSHCredentialOpts

// ScanRemoteHostKeyOpts identifies the host whose public host keys are to be
// discovered.
type ScanRemoteHostKeyOpts struct {
	Host           string
	Port           int64
	ConnectTimeout int64
}

// keychainCredentialResponse is the wire format returned by the
// keychaincredential.* methods. `attributes` differs per type, so it is left
// raw and decoded against the type the caller asked for.
type keychainCredentialResponse struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

type sshKeyPairAttributes struct {
	PublicKey string `json:"public_key"`
}

type sshCredentialAttributes struct {
	Host           string `json:"host"`
	Port           int64  `json:"port"`
	Username       string `json:"username"`
	PrivateKeyID   int64  `json:"private_key"`
	RemoteHostKey  string `json:"remote_host_key"`
	ConnectTimeout int64  `json:"connect_timeout"`
}

// KeychainCredentialService provides typed methods for the
// keychaincredential.* API namespace.
type KeychainCredentialService struct {
	client truenas.Caller
}

// NewKeychainCredentialService creates a new KeychainCredentialService.
func NewKeychainCredentialService(c truenas.Caller) *KeychainCredentialService {
	return &KeychainCredentialService{client: c}
}

// CreateSSHKeyPair creates an SSH key pair and returns it without its private
// half.
func (s *KeychainCredentialService) CreateSSHKeyPair(ctx context.Context, opts CreateSSHKeyPairOpts) (*SSHKeyPair, error) {
	result, err := s.client.Call(ctx, "keychaincredential.create", map[string]any{
		"name": opts.Name,
		"type": KeychainCredentialTypeSSHKeyPair,
		"attributes": map[string]any{
			"private_key": opts.PrivateKey,
		},
	})
	if err != nil {
		return nil, err
	}

	return decodeSSHKeyPair(result, "create")
}

// GetSSHKeyPair returns an SSH key pair by ID, or nil if it does not exist.
func (s *KeychainCredentialService) GetSSHKeyPair(ctx context.Context, id int64) (*SSHKeyPair, error) {
	result, err := s.client.Call(ctx, "keychaincredential.get_instance", id)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	return decodeSSHKeyPair(result, "get_instance")
}

// UpdateSSHKeyPair updates an SSH key pair and returns it without its private
// half.
func (s *KeychainCredentialService) UpdateSSHKeyPair(ctx context.Context, id int64, opts UpdateSSHKeyPairOpts) (*SSHKeyPair, error) {
	params := map[string]any{"name": opts.Name}
	if opts.PrivateKey != nil {
		params["attributes"] = map[string]any{"private_key": *opts.PrivateKey}
	}

	result, err := s.client.Call(ctx, "keychaincredential.update", []any{id, params})
	if err != nil {
		return nil, err
	}

	return decodeSSHKeyPair(result, "update")
}

// CreateSSHCredential creates an SSH connection and returns the full object.
func (s *KeychainCredentialService) CreateSSHCredential(ctx context.Context, opts CreateSSHCredentialOpts) (*SSHCredential, error) {
	result, err := s.client.Call(ctx, "keychaincredential.create", map[string]any{
		"name":       opts.Name,
		"type":       KeychainCredentialTypeSSHCredentials,
		"attributes": sshCredentialAttributeParams(opts),
	})
	if err != nil {
		return nil, err
	}

	return decodeSSHCredential(result, "create")
}

// GetSSHCredential returns an SSH connection by ID, or nil if it does not
// exist.
func (s *KeychainCredentialService) GetSSHCredential(ctx context.Context, id int64) (*SSHCredential, error) {
	result, err := s.client.Call(ctx, "keychaincredential.get_instance", id)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	return decodeSSHCredential(result, "get_instance")
}

// UpdateSSHCredential updates an SSH connection and returns the full object.
func (s *KeychainCredentialService) UpdateSSHCredential(ctx context.Context, id int64, opts UpdateSSHCredentialOpts) (*SSHCredential, error) {
	result, err := s.client.Call(ctx, "keychaincredential.update", []any{id, map[string]any{
		"name":       opts.Name,
		"attributes": sshCredentialAttributeParams(opts),
	}})
	if err != nil {
		return nil, err
	}

	return decodeSSHCredential(result, "update")
}

// Delete deletes a keychain credential by ID. The cascade option is not
// exposed: deleting a credential another object still references would silently
// delete or disable that object, which Terraform has no way to represent.
func (s *KeychainCredentialService) Delete(ctx context.Context, id int64) error {
	_, err := s.client.Call(ctx, "keychaincredential.delete", id)
	return err
}

// ScanRemoteHostKey returns the public host keys a host presents, in the format
// keychaincredential.create accepts for `remote_host_key`. The keys are
// accepted on trust: nothing has verified that the host answering is the
// intended one.
func (s *KeychainCredentialService) ScanRemoteHostKey(ctx context.Context, opts ScanRemoteHostKeyOpts) (string, error) {
	result, err := s.client.Call(ctx, "keychaincredential.remote_ssh_host_key_scan", map[string]any{
		"host":            opts.Host,
		"port":            opts.Port,
		"connect_timeout": opts.ConnectTimeout,
	})
	if err != nil {
		return "", err
	}

	var hostKey string
	if err := json.Unmarshal(result, &hostKey); err != nil {
		return "", fmt.Errorf("parse remote_ssh_host_key_scan response: %w", err)
	}
	return hostKey, nil
}

// sshCredentialAttributeParams converts SSH connection options to the API's
// `attributes` object, which create and update share.
func sshCredentialAttributeParams(opts CreateSSHCredentialOpts) map[string]any {
	return map[string]any{
		"host":            opts.Host,
		"port":            opts.Port,
		"username":        opts.Username,
		"private_key":     opts.PrivateKeyID,
		"remote_host_key": opts.RemoteHostKey,
		"connect_timeout": opts.ConnectTimeout,
	}
}

// decodeSSHKeyPair converts a keychaincredential.* response to an SSHKeyPair,
// rejecting a credential of the wrong type rather than reporting empty fields.
func decodeSSHKeyPair(result json.RawMessage, method string) (*SSHKeyPair, error) {
	resp, err := decodeKeychainCredential(result, method, KeychainCredentialTypeSSHKeyPair)
	if err != nil {
		return nil, err
	}

	var attrs sshKeyPairAttributes
	if err := json.Unmarshal(resp.Attributes, &attrs); err != nil {
		return nil, fmt.Errorf("parse %s attributes: %w", method, err)
	}

	return &SSHKeyPair{
		ID:        resp.ID,
		Name:      resp.Name,
		PublicKey: attrs.PublicKey,
	}, nil
}

// decodeSSHCredential converts a keychaincredential.* response to an
// SSHCredential, rejecting a credential of the wrong type rather than
// reporting empty fields.
func decodeSSHCredential(result json.RawMessage, method string) (*SSHCredential, error) {
	resp, err := decodeKeychainCredential(result, method, KeychainCredentialTypeSSHCredentials)
	if err != nil {
		return nil, err
	}

	var attrs sshCredentialAttributes
	if err := json.Unmarshal(resp.Attributes, &attrs); err != nil {
		return nil, fmt.Errorf("parse %s attributes: %w", method, err)
	}

	return &SSHCredential{
		ID:             resp.ID,
		Name:           resp.Name,
		Host:           attrs.Host,
		Port:           attrs.Port,
		Username:       attrs.Username,
		PrivateKeyID:   attrs.PrivateKeyID,
		RemoteHostKey:  attrs.RemoteHostKey,
		ConnectTimeout: attrs.ConnectTimeout,
	}, nil
}

// decodeKeychainCredential parses a response and asserts its type. The keychain
// holds both credential types under one set of endpoints, so an ID naming the
// other type has to be caught here — most likely on an import of the wrong ID.
func decodeKeychainCredential(result json.RawMessage, method, want string) (*keychainCredentialResponse, error) {
	var resp keychainCredentialResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse %s response: %w", method, err)
	}

	if resp.Type != want {
		return nil, fmt.Errorf("keychain credential %d is of type %s, not %s", resp.ID, resp.Type, want)
	}

	return &resp, nil
}
