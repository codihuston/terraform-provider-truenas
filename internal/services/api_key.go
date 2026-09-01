package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	truenas "github.com/deevus/truenas-go"
)

// APIKey is the user-facing representation of a TrueNAS API key
// (the `api_key` API namespace).
type APIKey struct {
	ID   int64
	Name string
	// Username is the account the key authenticates as. It is nil for the
	// system-owned keys the API creates for itself.
	Username *string
	// UserIdentifier is the numeric UID or the SID of the owning account.
	UserIdentifier string
	CreatedAt      time.Time
	// ExpiresAt is nil when the key never expires.
	ExpiresAt *time.Time
	Local     bool
	Revoked   bool
	// RevokedReason is nil unless the key has been revoked.
	RevokedReason *string
	// Key is the secret used to authenticate. The API discloses it only in the
	// reply to Create, so it is empty on every other operation.
	Key string
}

// CreateAPIKeyOpts contains options for creating an API key.
type CreateAPIKeyOpts struct {
	Name     string
	Username string
	// ExpiresAt is nil for a key that never expires.
	ExpiresAt *time.Time
}

// UpdateAPIKeyOpts contains options for updating an API key. The owning
// username cannot be changed; only a new key replaces it.
type UpdateAPIKeyOpts struct {
	Name string
	// ExpiresAt is nil for a key that never expires.
	ExpiresAt *time.Time
}

// apiKeyResponse is the wire format returned by the api_key.* methods.
type apiKeyResponse struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Username       *string        `json:"username"`
	UserIdentifier userIdentifier `json:"user_identifier"`
	CreatedAt      apiTime        `json:"created_at"`
	ExpiresAt      *apiTime       `json:"expires_at"`
	Local          bool           `json:"local"`
	Revoked        bool           `json:"revoked"`
	RevokedReason  *string        `json:"revoked_reason"`
	Key            string         `json:"key"`
}

// apiTime is the wire representation of a TrueNAS timestamp: an object wrapping
// milliseconds since the Unix epoch, as in `{"$date": 1788287836000}`.
type apiTime struct {
	time.Time
}

// MarshalJSON renders the instant in the wrapper the API expects. A plain
// RFC 3339 string is rejected with EINVAL.
func (t apiTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]int64{"$date": t.UnixMilli()})
}

// UnmarshalJSON reads the millisecond wrapper into a UTC instant.
func (t *apiTime) UnmarshalJSON(data []byte) error {
	var wrapper struct {
		Date *int64 `json:"$date"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse timestamp %s: %w", data, err)
	}
	if wrapper.Date == nil {
		return fmt.Errorf("parse timestamp %s: missing $date", data)
	}

	t.Time = time.UnixMilli(*wrapper.Date).UTC()
	return nil
}

// userIdentifier is the wire representation of `user_identifier`, which the API
// returns as a number for a local UID and as a string for a directory SID.
type userIdentifier string

// UnmarshalJSON accepts either representation and keeps the text form.
func (u *userIdentifier) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*u = userIdentifier(str)
		return nil
	}

	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return fmt.Errorf("parse user_identifier %s: %w", data, err)
	}

	*u = userIdentifier(num.String())
	return nil
}

// APIKeyService provides typed methods for the api_key.* API namespace.
type APIKeyService struct {
	client truenas.Caller
}

// NewAPIKeyService creates a new APIKeyService.
func NewAPIKeyService(c truenas.Caller) *APIKeyService {
	return &APIKeyService{client: c}
}

// Create creates an API key. The returned key is the only time the API
// discloses the secret.
func (s *APIKeyService) Create(ctx context.Context, opts CreateAPIKeyOpts) (*APIKey, error) {
	result, err := s.client.Call(ctx, "api_key.create", map[string]any{
		"name":       opts.Name,
		"username":   opts.Username,
		"expires_at": apiTimePointer(opts.ExpiresAt),
	})
	if err != nil {
		return nil, err
	}

	return parseAPIKeyResponse("create", result)
}

// Get returns an API key by ID, or nil if it does not exist. The secret is
// never part of the reply.
func (s *APIKeyService) Get(ctx context.Context, id int64) (*APIKey, error) {
	result, err := s.client.Call(ctx, "api_key.get_instance", id)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	return parseAPIKeyResponse("get_instance", result)
}

// List returns all API keys visible to the caller.
func (s *APIKeyService) List(ctx context.Context) ([]APIKey, error) {
	result, err := s.client.Call(ctx, "api_key.query", nil)
	if err != nil {
		return nil, err
	}

	var responses []apiKeyResponse
	if err := json.Unmarshal(result, &responses); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}

	keys := make([]APIKey, len(responses))
	for i, resp := range responses {
		keys[i] = apiKeyFromResponse(resp)
	}
	return keys, nil
}

// Update updates an API key. The reply never carries the secret.
func (s *APIKeyService) Update(ctx context.Context, id int64, opts UpdateAPIKeyOpts) (*APIKey, error) {
	result, err := s.client.Call(ctx, "api_key.update", []any{id, map[string]any{
		"name":       opts.Name,
		"expires_at": apiTimePointer(opts.ExpiresAt),
	}})
	if err != nil {
		return nil, err
	}

	return parseAPIKeyResponse("update", result)
}

// Delete deletes an API key by ID.
func (s *APIKeyService) Delete(ctx context.Context, id int64) error {
	_, err := s.client.Call(ctx, "api_key.delete", id)
	return err
}

// apiTimePointer wraps an optional instant for the wire, preserving nil so the
// API receives an explicit null.
func apiTimePointer(t *time.Time) *apiTime {
	if t == nil {
		return nil
	}
	return &apiTime{Time: *t}
}

// parseAPIKeyResponse decodes a single-key reply from the named method.
func parseAPIKeyResponse(method string, result json.RawMessage) (*APIKey, error) {
	var resp apiKeyResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse %s response: %w", method, err)
	}

	key := apiKeyFromResponse(resp)
	return &key, nil
}

// apiKeyFromResponse converts a wire-format response to a user-facing APIKey.
func apiKeyFromResponse(resp apiKeyResponse) APIKey {
	key := APIKey{
		ID:             resp.ID,
		Name:           resp.Name,
		Username:       resp.Username,
		UserIdentifier: string(resp.UserIdentifier),
		CreatedAt:      resp.CreatedAt.Time,
		Local:          resp.Local,
		Revoked:        resp.Revoked,
		RevokedReason:  resp.RevokedReason,
		Key:            resp.Key,
	}

	if resp.ExpiresAt != nil {
		expiresAt := resp.ExpiresAt.Time
		key.ExpiresAt = &expiresAt
	}

	return key
}
