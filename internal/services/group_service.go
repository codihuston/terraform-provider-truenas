package services

import (
	"context"
	"encoding/json"
	"fmt"

	truenas "github.com/deevus/truenas-go"
)

// GroupResponse represents a group as returned by the group.* API.
type GroupResponse struct {
	ID                   int64    `json:"id"`
	GID                  int64    `json:"gid"`
	Name                 string   `json:"name"`
	SMB                  bool     `json:"smb"`
	SudoCommands         []string `json:"sudo_commands"`
	SudoCommandsNoPasswd []string `json:"sudo_commands_nopasswd"`
	Builtin              bool     `json:"builtin"`
}

// Group is the user-facing representation of a TrueNAS group.
type Group struct {
	ID                   int64
	GID                  int64
	Name                 string
	SMB                  bool
	SudoCommands         []string
	SudoCommandsNoPasswd []string
	Builtin              bool
}

// CreateGroupOpts contains options for creating a group. GID is omitted from
// the request when nil so the server allocates the next available one.
type CreateGroupOpts struct {
	Name                 string
	GID                  *int64
	SMB                  bool
	SudoCommands         []string
	SudoCommandsNoPasswd []string
}

// UpdateGroupOpts contains options for updating a group. The API rejects `gid`
// on update, so it is absent here.
type UpdateGroupOpts struct {
	Name                 string
	SMB                  bool
	SudoCommands         []string
	SudoCommandsNoPasswd []string
}

// GroupService provides typed methods for the group.* API namespace.
type GroupService struct {
	client truenas.Caller
}

// NewGroupService creates a new GroupService.
func NewGroupService(c truenas.Caller) *GroupService {
	return &GroupService{client: c}
}

// Create creates a group and returns the full object. group.create responds
// with the new entry ID alone, so the group is read back.
func (s *GroupService) Create(ctx context.Context, opts CreateGroupOpts) (*Group, error) {
	params := map[string]any{
		"name":                   opts.Name,
		"smb":                    opts.SMB,
		"sudo_commands":          nonNil(opts.SudoCommands),
		"sudo_commands_nopasswd": nonNil(opts.SudoCommandsNoPasswd),
	}
	if opts.GID != nil {
		params["gid"] = *opts.GID
	}

	result, err := s.client.Call(ctx, "group.create", params)
	if err != nil {
		return nil, err
	}

	var id int64
	if err := json.Unmarshal(result, &id); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}

	return s.getCreated(ctx, id)
}

// Get returns a group by ID, or nil if it does not exist.
func (s *GroupService) Get(ctx context.Context, id int64) (*Group, error) {
	result, err := s.client.Call(ctx, "group.get_instance", id)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	var resp GroupResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse get_instance response: %w", err)
	}

	group := groupFromResponse(resp)
	return &group, nil
}

// Update updates a group and returns the full object. group.update responds
// with the entry ID alone, so the group is read back.
func (s *GroupService) Update(ctx context.Context, id int64, opts UpdateGroupOpts) (*Group, error) {
	params := map[string]any{
		"name":                   opts.Name,
		"smb":                    opts.SMB,
		"sudo_commands":          nonNil(opts.SudoCommands),
		"sudo_commands_nopasswd": nonNil(opts.SudoCommandsNoPasswd),
	}

	if _, err := s.client.Call(ctx, "group.update", []any{id, params}); err != nil {
		return nil, err
	}

	return s.getCreated(ctx, id)
}

// getCreated reads back a group that the API has just written, treating a
// missing group as an error rather than as a deletion.
func (s *GroupService) getCreated(ctx context.Context, id int64) (*Group, error) {
	group, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, fmt.Errorf("group %d not found after write", id)
	}

	return group, nil
}

// Delete deletes a group by ID. Members keep their accounts; only users whose
// primary group this is are affected, and TrueNAS refuses the call in that case.
func (s *GroupService) Delete(ctx context.Context, id int64) error {
	_, err := s.client.Call(ctx, "group.delete", []any{id, map[string]any{"delete_users": false}})
	return err
}

// groupFromResponse converts a wire-format GroupResponse to a user-facing Group.
func groupFromResponse(resp GroupResponse) Group {
	return Group{
		ID:                   resp.ID,
		GID:                  resp.GID,
		Name:                 resp.Name,
		SMB:                  resp.SMB,
		SudoCommands:         nonNil(resp.SudoCommands),
		SudoCommandsNoPasswd: nonNil(resp.SudoCommandsNoPasswd),
		Builtin:              resp.Builtin,
	}
}
