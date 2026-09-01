package services

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

// testAPIKeyJSON is an api_key.create reply, the only one that carries the
// secret. created_at and expires_at use the API's millisecond wrapper.
const testAPIKeyJSON = `{
	"id": 2,
	"name": "terraform",
	"username": "truenas_admin",
	"user_identifier": "33",
	"keyhash": "$pbkdf2-sha512$500000$c2FsdA==$aGFzaA==",
	"created_at": {"$date": 1788287836000},
	"expires_at": {"$date": 1893553445000},
	"local": true,
	"revoked": false,
	"revoked_reason": null,
	"key": "2-amfpg5iQSKI5rClsylOGH09wZnCvcmsKUJdGxu9yUddRkQWewAW21fgLwdowOiVy"
}`

// testAPIKeyQueryJSON is an api_key.query entry: no secret, a numeric
// user_identifier and no expiry.
const testAPIKeyQueryJSON = `{
	"id": 2,
	"name": "terraform",
	"username": "truenas_admin",
	"user_identifier": 33,
	"created_at": {"$date": 1788287836000},
	"expires_at": null,
	"local": true,
	"revoked": true,
	"revoked_reason": "EXPIRED"
}`

func testExpiry() time.Time {
	return time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
}

func assertAPIKey(t *testing.T, key *APIKey) {
	t.Helper()

	if key == nil {
		t.Fatal("expected key, got nil")
	}
	if key.ID != 2 {
		t.Errorf("expected ID 2, got %d", key.ID)
	}
	if key.Name != "terraform" {
		t.Errorf("expected name 'terraform', got %q", key.Name)
	}
	if key.Username == nil || *key.Username != "truenas_admin" {
		t.Errorf("expected username 'truenas_admin', got %v", key.Username)
	}
	if key.UserIdentifier != "33" {
		t.Errorf("expected user_identifier '33', got %q", key.UserIdentifier)
	}
	if want := time.UnixMilli(1788287836000).UTC(); !key.CreatedAt.Equal(want) {
		t.Errorf("expected created_at %s, got %s", want, key.CreatedAt)
	}
	if key.ExpiresAt == nil || !key.ExpiresAt.Equal(testExpiry()) {
		t.Errorf("expected expires_at %s, got %v", testExpiry(), key.ExpiresAt)
	}
	if !key.Local {
		t.Error("expected local true")
	}
	if key.Revoked {
		t.Error("expected revoked false")
	}
	if key.RevokedReason != nil {
		t.Errorf("expected nil revoked_reason, got %q", *key.RevokedReason)
	}
}

func TestAPIKeyService_Create(t *testing.T) {
	c := &fakeCaller{result: testAPIKeyJSON}
	s := NewAPIKeyService(c)

	expiry := testExpiry()
	key, err := s.Create(context.Background(), CreateAPIKeyOpts{
		Name:      "terraform",
		Username:  "truenas_admin",
		ExpiresAt: &expiry,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "api_key.create" {
		t.Errorf("expected method 'api_key.create', got %q", c.method)
	}

	params, ok := c.params.(map[string]any)
	if !ok {
		t.Fatalf("expected map params, got %T", c.params)
	}
	if params["name"] != "terraform" {
		t.Errorf("expected name 'terraform', got %v", params["name"])
	}
	if params["username"] != "truenas_admin" {
		t.Errorf("expected username 'truenas_admin', got %v", params["username"])
	}
	assertMarshalsTo(t, params["expires_at"], `{"$date":1893553445000}`)

	assertAPIKey(t, key)
	if key.Key == "" {
		t.Error("expected the create reply to carry the secret")
	}
}

func TestAPIKeyService_Create_NoExpiry(t *testing.T) {
	c := &fakeCaller{result: testAPIKeyQueryJSON}
	s := NewAPIKeyService(c)

	key, err := s.Create(context.Background(), CreateAPIKeyOpts{Name: "terraform", Username: "truenas_admin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := c.params.(map[string]any)
	assertMarshalsTo(t, params["expires_at"], `null`)

	if key.ExpiresAt != nil {
		t.Errorf("expected nil expires_at, got %s", key.ExpiresAt)
	}
	if key.Key != "" {
		t.Errorf("expected no secret outside a create reply, got %q", key.Key)
	}
}

func TestAPIKeyService_Create_Error(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.Create(context.Background(), CreateAPIKeyOpts{Name: "terraform", Username: "root"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestAPIKeyService_Create_BadJSON(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{result: `not json`})

	if _, err := s.Create(context.Background(), CreateAPIKeyOpts{Name: "terraform", Username: "root"}); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestAPIKeyService_Get(t *testing.T) {
	c := &fakeCaller{result: testAPIKeyJSON}
	s := NewAPIKeyService(c)

	key, err := s.Get(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "api_key.get_instance" {
		t.Errorf("expected method 'api_key.get_instance', got %q", c.method)
	}
	if c.params != int64(2) {
		t.Errorf("expected params 2, got %v", c.params)
	}
	assertAPIKey(t, key)
}

func TestAPIKeyService_Get_NotFound(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{err: enoentRPCError()})

	key, err := s.Get(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != nil {
		t.Fatalf("expected nil key, got %+v", key)
	}
}

func TestAPIKeyService_Get_Error(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.Get(context.Background(), 2); err == nil {
		t.Fatal("expected error")
	}
}

func TestAPIKeyService_Get_BadJSON(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{result: `not json`})

	if _, err := s.Get(context.Background(), 2); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestAPIKeyService_List(t *testing.T) {
	c := &fakeCaller{result: "[" + testAPIKeyQueryJSON + "]"}
	s := NewAPIKeyService(c)

	keys, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "api_key.query" {
		t.Errorf("expected method 'api_key.query', got %q", c.method)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	// The query reply spells user_identifier as a number.
	if keys[0].UserIdentifier != "33" {
		t.Errorf("expected user_identifier '33', got %q", keys[0].UserIdentifier)
	}
	if !keys[0].Revoked {
		t.Error("expected revoked true")
	}
	if keys[0].RevokedReason == nil || *keys[0].RevokedReason != "EXPIRED" {
		t.Errorf("expected revoked_reason 'EXPIRED', got %v", keys[0].RevokedReason)
	}
}

func TestAPIKeyService_List_Error(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestAPIKeyService_List_BadJSON(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{result: `not json`})

	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestAPIKeyService_Update(t *testing.T) {
	c := &fakeCaller{result: testAPIKeyQueryJSON}
	s := NewAPIKeyService(c)

	expiry := testExpiry()
	key, err := s.Update(context.Background(), 2, UpdateAPIKeyOpts{Name: "renamed", ExpiresAt: &expiry})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "api_key.update" {
		t.Errorf("expected method 'api_key.update', got %q", c.method)
	}

	params, ok := c.params.([]any)
	if !ok || len(params) != 2 {
		t.Fatalf("expected [id, body] params, got %#v", c.params)
	}
	if params[0] != int64(2) {
		t.Errorf("expected id 2, got %v", params[0])
	}

	body := params[1].(map[string]any)
	if body["name"] != "renamed" {
		t.Errorf("expected name 'renamed', got %v", body["name"])
	}
	if _, ok := body["username"]; ok {
		t.Error("api_key.update rejects username; it must not be sent")
	}
	assertMarshalsTo(t, body["expires_at"], `{"$date":1893553445000}`)

	if key.Key != "" {
		t.Errorf("expected no secret in an update reply, got %q", key.Key)
	}
}

func TestAPIKeyService_Update_Error(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{err: errors.New("boom")})

	if _, err := s.Update(context.Background(), 2, UpdateAPIKeyOpts{Name: "renamed"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestAPIKeyService_Update_BadJSON(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{result: `not json`})

	if _, err := s.Update(context.Background(), 2, UpdateAPIKeyOpts{Name: "renamed"}); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestAPIKeyService_Delete(t *testing.T) {
	c := &fakeCaller{result: `true`}
	s := NewAPIKeyService(c)

	if err := s.Delete(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.method != "api_key.delete" {
		t.Errorf("expected method 'api_key.delete', got %q", c.method)
	}
	if c.params != int64(2) {
		t.Errorf("expected params 2, got %v", c.params)
	}
}

func TestAPIKeyService_Delete_Error(t *testing.T) {
	s := NewAPIKeyService(&fakeCaller{err: errors.New("boom")})

	if err := s.Delete(context.Background(), 2); err == nil {
		t.Fatal("expected error")
	}
}

func TestAPITime_UnmarshalJSON_Invalid(t *testing.T) {
	tests := map[string]string{
		"not an object": `"2030-01-02T03:04:05Z"`,
		"missing $date": `{}`,
		"null $date":    `{"$date": null}`,
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			var got apiTime
			if err := json.Unmarshal([]byte(input), &got); err == nil {
				t.Fatalf("expected error for %s", input)
			}
		})
	}
}

func TestUserIdentifier_UnmarshalJSON(t *testing.T) {
	tests := map[string]struct {
		input   string
		want    userIdentifier
		wantErr bool
	}{
		"numeric uid": {input: `33`, want: "33"},
		"directory sid": {
			input: `"S-1-5-21-1004336348-1177238915-682003330-512"`,
			want:  "S-1-5-21-1004336348-1177238915-682003330-512",
		},
		"unsupported": {input: `{}`, wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got userIdentifier
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestMockAPIKeyService_Defaults(t *testing.T) {
	var m APIKeyServiceAPI = &MockAPIKeyService{}
	ctx := context.Background()

	if key, err := m.Create(ctx, CreateAPIKeyOpts{}); key != nil || err != nil {
		t.Errorf("expected zero values, got %v, %v", key, err)
	}
	if key, err := m.Get(ctx, 1); key != nil || err != nil {
		t.Errorf("expected zero values, got %v, %v", key, err)
	}
	if keys, err := m.List(ctx); keys != nil || err != nil {
		t.Errorf("expected zero values, got %v, %v", keys, err)
	}
	if key, err := m.Update(ctx, 1, UpdateAPIKeyOpts{}); key != nil || err != nil {
		t.Errorf("expected zero values, got %v, %v", key, err)
	}
	if err := m.Delete(ctx, 1); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestMockAPIKeyService_Delegates(t *testing.T) {
	want := &APIKey{ID: 2}
	m := &MockAPIKeyService{
		CreateFunc: func(ctx context.Context, opts CreateAPIKeyOpts) (*APIKey, error) { return want, nil },
		GetFunc:    func(ctx context.Context, id int64) (*APIKey, error) { return want, nil },
		ListFunc:   func(ctx context.Context) ([]APIKey, error) { return []APIKey{*want}, nil },
		UpdateFunc: func(ctx context.Context, id int64, opts UpdateAPIKeyOpts) (*APIKey, error) { return want, nil },
		DeleteFunc: func(ctx context.Context, id int64) error { return errors.New("boom") },
	}
	ctx := context.Background()

	if got, _ := m.Create(ctx, CreateAPIKeyOpts{}); got != want {
		t.Errorf("Create did not delegate")
	}
	if got, _ := m.Get(ctx, 2); got != want {
		t.Errorf("Get did not delegate")
	}
	if got, _ := m.List(ctx); !reflect.DeepEqual(got, []APIKey{*want}) {
		t.Errorf("List did not delegate")
	}
	if got, _ := m.Update(ctx, 2, UpdateAPIKeyOpts{}); got != want {
		t.Errorf("Update did not delegate")
	}
	if err := m.Delete(ctx, 2); err == nil {
		t.Errorf("Delete did not delegate")
	}
}

// assertMarshalsTo checks the JSON a request parameter serialises to, which is
// what the API actually sees.
func assertMarshalsTo(t *testing.T, value any, want string) {
	t.Helper()

	got, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %#v: %v", value, err)
	}
	if string(got) != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}
