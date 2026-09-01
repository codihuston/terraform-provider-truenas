package services

import (
	"context"
	"encoding/json"
	"fmt"

	truenas "github.com/deevus/truenas-go"
)

// UserResponse represents a user account as returned by the user.* API.
type UserResponse struct {
	ID                   int64             `json:"id"`
	UID                  int64             `json:"uid"`
	Username             string            `json:"username"`
	FullName             string            `json:"full_name"`
	Email                *string           `json:"email"`
	Home                 string            `json:"home"`
	Shell                string            `json:"shell"`
	Group                UserGroupResponse `json:"group"`
	Groups               []int64           `json:"groups"`
	SMB                  bool              `json:"smb"`
	Locked               bool              `json:"locked"`
	PasswordDisabled     bool              `json:"password_disabled"`
	SSHPasswordEnabled   bool              `json:"ssh_password_enabled"`
	SSHPubKey            *string           `json:"sshpubkey"`
	SudoCommands         []string          `json:"sudo_commands"`
	SudoCommandsNoPasswd []string          `json:"sudo_commands_nopasswd"`
	Builtin              bool              `json:"builtin"`
}

// UserGroupResponse is the primary group embedded in a user record. Only the
// group entry `id` is meaningful to the create and update endpoints, which take
// the entry id rather than the Unix gid.
type UserGroupResponse struct {
	ID  int64 `json:"id"`
	GID int64 `json:"bsdgrp_gid"`
}

// User is the user-facing representation of a TrueNAS user account.
type User struct {
	ID                   int64
	UID                  int64
	Username             string
	FullName             string
	Email                string
	Home                 string
	Shell                string
	Group                int64
	GroupGID             int64
	Groups               []int64
	SMB                  bool
	Locked               bool
	PasswordDisabled     bool
	SSHPasswordEnabled   bool
	SSHPublicKey         string
	SudoCommands         []string
	SudoCommandsNoPasswd []string
	Builtin              bool
}

// CreateUserOpts contains options for creating a user account.
//
// Pointer fields are omitted from the request when nil, which lets the caller
// distinguish "leave unset" from an explicit zero value. UID is omitted so the
// server allocates the next available one, and Password is omitted so the
// account is created without password authentication.
type CreateUserOpts struct {
	Username             string
	FullName             string
	UID                  *int64
	Group                *int64
	GroupCreate          bool
	Groups               []int64
	Home                 string
	HomeCreate           bool
	HomeMode             string
	Shell                string
	Email                *string
	SMB                  bool
	Locked               bool
	PasswordDisabled     bool
	SSHPasswordEnabled   bool
	SSHPublicKey         *string
	SudoCommands         []string
	SudoCommandsNoPasswd []string
	Password             *string
}

// UpdateUserOpts contains options for updating a user account.
//
// The API rejects `uid` and `group_create` on update, so neither is present
// here. HomeCreate is only honoured when Home changes; sending it against an
// unchanged path makes TrueNAS nest a fresh directory inside the existing home.
type UpdateUserOpts struct {
	Username             string
	FullName             string
	Group                *int64
	Groups               []int64
	Home                 string
	HomeCreate           bool
	HomeMode             string
	Shell                string
	Email                *string
	SMB                  bool
	Locked               bool
	PasswordDisabled     bool
	SSHPasswordEnabled   bool
	SSHPublicKey         *string
	SudoCommands         []string
	SudoCommandsNoPasswd []string
	Password             *string
}

// UserService provides typed methods for the user.* API namespace.
type UserService struct {
	client truenas.Caller
}

// NewUserService creates a new UserService.
func NewUserService(c truenas.Caller) *UserService {
	return &UserService{client: c}
}

// Create creates a user account and returns the full object.
func (s *UserService) Create(ctx context.Context, opts CreateUserOpts) (*User, error) {
	result, err := s.client.Call(ctx, "user.create", createUserParams(opts))
	if err != nil {
		return nil, err
	}

	var resp UserResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}

	user := userFromResponse(resp)
	return &user, nil
}

// Get returns a user account by ID, or nil if it does not exist.
func (s *UserService) Get(ctx context.Context, id int64) (*User, error) {
	result, err := s.client.Call(ctx, "user.get_instance", id)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	var resp UserResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse get_instance response: %w", err)
	}

	user := userFromResponse(resp)
	return &user, nil
}

// Update updates a user account and returns the full object.
func (s *UserService) Update(ctx context.Context, id int64, opts UpdateUserOpts) (*User, error) {
	result, err := s.client.Call(ctx, "user.update", []any{id, updateUserParams(opts)})
	if err != nil {
		return nil, err
	}

	var resp UserResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse update response: %w", err)
	}

	user := userFromResponse(resp)
	return &user, nil
}

// Delete deletes a user account by ID. When deleteGroup is true the primary
// group is removed as well, provided no other user depends on it.
func (s *UserService) Delete(ctx context.Context, id int64, deleteGroup bool) error {
	_, err := s.client.Call(ctx, "user.delete", []any{id, map[string]any{"delete_group": deleteGroup}})
	return err
}

// createUserParams converts CreateUserOpts to API parameters.
func createUserParams(opts CreateUserOpts) map[string]any {
	params := map[string]any{
		"username":               opts.Username,
		"full_name":              opts.FullName,
		"group_create":           opts.GroupCreate,
		"groups":                 nonNil(opts.Groups),
		"home":                   opts.Home,
		"home_create":            opts.HomeCreate,
		"home_mode":              opts.HomeMode,
		"shell":                  opts.Shell,
		"email":                  opts.Email,
		"smb":                    opts.SMB,
		"locked":                 opts.Locked,
		"password_disabled":      opts.PasswordDisabled,
		"ssh_password_enabled":   opts.SSHPasswordEnabled,
		"sshpubkey":              opts.SSHPublicKey,
		"sudo_commands":          nonNil(opts.SudoCommands),
		"sudo_commands_nopasswd": nonNil(opts.SudoCommandsNoPasswd),
	}

	if opts.UID != nil {
		params["uid"] = *opts.UID
	}
	if opts.Group != nil {
		params["group"] = *opts.Group
	}
	if opts.Password != nil {
		params["password"] = *opts.Password
	}

	return params
}

// updateUserParams converts UpdateUserOpts to API parameters.
func updateUserParams(opts UpdateUserOpts) map[string]any {
	params := map[string]any{
		"username":               opts.Username,
		"full_name":              opts.FullName,
		"groups":                 nonNil(opts.Groups),
		"home":                   opts.Home,
		"home_mode":              opts.HomeMode,
		"shell":                  opts.Shell,
		"email":                  opts.Email,
		"smb":                    opts.SMB,
		"locked":                 opts.Locked,
		"password_disabled":      opts.PasswordDisabled,
		"ssh_password_enabled":   opts.SSHPasswordEnabled,
		"sshpubkey":              opts.SSHPublicKey,
		"sudo_commands":          nonNil(opts.SudoCommands),
		"sudo_commands_nopasswd": nonNil(opts.SudoCommandsNoPasswd),
	}

	if opts.HomeCreate {
		params["home_create"] = true
	}
	if opts.Group != nil {
		params["group"] = *opts.Group
	}
	if opts.Password != nil {
		params["password"] = *opts.Password
	}

	return params
}

// userFromResponse converts a wire-format UserResponse to a user-facing User.
func userFromResponse(resp UserResponse) User {
	user := User{
		ID:                   resp.ID,
		UID:                  resp.UID,
		Username:             resp.Username,
		FullName:             resp.FullName,
		Home:                 resp.Home,
		Shell:                resp.Shell,
		Group:                resp.Group.ID,
		GroupGID:             resp.Group.GID,
		Groups:               nonNil(resp.Groups),
		SMB:                  resp.SMB,
		Locked:               resp.Locked,
		PasswordDisabled:     resp.PasswordDisabled,
		SSHPasswordEnabled:   resp.SSHPasswordEnabled,
		SudoCommands:         nonNil(resp.SudoCommands),
		SudoCommandsNoPasswd: nonNil(resp.SudoCommandsNoPasswd),
		Builtin:              resp.Builtin,
	}

	if resp.Email != nil {
		user.Email = *resp.Email
	}
	if resp.SSHPubKey != nil {
		user.SSHPublicKey = *resp.SSHPubKey
	}

	return user
}

// nonNil returns an empty slice in place of a nil one so the API receives an
// empty JSON array rather than null, and Terraform sees an empty collection
// rather than an unexpected null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
