package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	truenas "github.com/deevus/truenas-go"
	"github.com/deevus/truenas-go/client"
)

// errnoENOENT is the POSIX ENOENT value the API reports in JSON-RPC error data
// when an instance does not exist.
const errnoENOENT = 2

// NFSShare is the user-facing representation of a TrueNAS NFS share
// (the `sharing.nfs` API namespace).
type NFSShare struct {
	ID              int64
	Path            string
	Aliases         []string
	Comment         string
	Networks        []string
	Hosts           []string
	ReadOnly        bool
	MaprootUser     *string
	MaprootGroup    *string
	MapallUser      *string
	MapallGroup     *string
	Security        []string
	Enabled         bool
	ExposeSnapshots bool
	// Locked reports whether the share lives on a locked dataset. It is
	// read-only and may be nil when the API did not compute lock status.
	Locked *bool
}

// CreateNFSShareOpts contains options for creating an NFS share.
// All fields are always sent on create.
type CreateNFSShareOpts struct {
	Path            string
	Comment         string
	Networks        []string
	Hosts           []string
	ReadOnly        bool
	MaprootUser     *string
	MaprootGroup    *string
	MapallUser      *string
	MapallGroup     *string
	Security        []string
	Enabled         bool
	ExposeSnapshots bool
}

// UpdateNFSShareOpts contains options for updating an NFS share.
// All fields are always sent on update.
type UpdateNFSShareOpts = CreateNFSShareOpts

// nfsShareResponse is the wire format returned by the sharing.nfs.* methods.
type nfsShareResponse struct {
	ID              int64    `json:"id"`
	Path            string   `json:"path"`
	Aliases         []string `json:"aliases"`
	Comment         string   `json:"comment"`
	Networks        []string `json:"networks"`
	Hosts           []string `json:"hosts"`
	RO              bool     `json:"ro"`
	MaprootUser     *string  `json:"maproot_user"`
	MaprootGroup    *string  `json:"maproot_group"`
	MapallUser      *string  `json:"mapall_user"`
	MapallGroup     *string  `json:"mapall_group"`
	Security        []string `json:"security"`
	Enabled         bool     `json:"enabled"`
	ExposeSnapshots bool     `json:"expose_snapshots"`
	Locked          *bool    `json:"locked"`
}

// SharingNFSService provides typed methods for the sharing.nfs.* API namespace.
type SharingNFSService struct {
	client truenas.Caller
}

// NewSharingNFSService creates a new SharingNFSService.
func NewSharingNFSService(c truenas.Caller) *SharingNFSService {
	return &SharingNFSService{client: c}
}

// Create creates an NFS share and returns the full object.
func (s *SharingNFSService) Create(ctx context.Context, opts CreateNFSShareOpts) (*NFSShare, error) {
	result, err := s.client.Call(ctx, "sharing.nfs.create", nfsOptsToParams(opts))
	if err != nil {
		return nil, err
	}

	var resp nfsShareResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}

	share := nfsShareFromResponse(resp)
	return &share, nil
}

// Get returns an NFS share by ID, or nil if it does not exist.
func (s *SharingNFSService) Get(ctx context.Context, id int64) (*NFSShare, error) {
	result, err := s.client.Call(ctx, "sharing.nfs.get_instance", id)
	if err != nil {
		if isNFSNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	var resp nfsShareResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse get_instance response: %w", err)
	}

	share := nfsShareFromResponse(resp)
	return &share, nil
}

// List returns all NFS shares.
func (s *SharingNFSService) List(ctx context.Context) ([]NFSShare, error) {
	result, err := s.client.Call(ctx, "sharing.nfs.query", nil)
	if err != nil {
		return nil, err
	}

	var responses []nfsShareResponse
	if err := json.Unmarshal(result, &responses); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}

	shares := make([]NFSShare, len(responses))
	for i, resp := range responses {
		shares[i] = nfsShareFromResponse(resp)
	}
	return shares, nil
}

// Update updates an NFS share and returns the full object.
func (s *SharingNFSService) Update(ctx context.Context, id int64, opts UpdateNFSShareOpts) (*NFSShare, error) {
	result, err := s.client.Call(ctx, "sharing.nfs.update", []any{id, nfsOptsToParams(opts)})
	if err != nil {
		return nil, err
	}

	var resp nfsShareResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse update response: %w", err)
	}

	share := nfsShareFromResponse(resp)
	return &share, nil
}

// Delete deletes an NFS share by ID.
func (s *SharingNFSService) Delete(ctx context.Context, id int64) error {
	_, err := s.client.Call(ctx, "sharing.nfs.delete", id)
	return err
}

// nfsOptsToParams converts CreateNFSShareOpts to API parameters.
func nfsOptsToParams(opts CreateNFSShareOpts) map[string]any {
	return map[string]any{
		"path":             opts.Path,
		"comment":          opts.Comment,
		"networks":         nfsStringList(opts.Networks),
		"hosts":            nfsStringList(opts.Hosts),
		"ro":               opts.ReadOnly,
		"maproot_user":     opts.MaprootUser,
		"maproot_group":    opts.MaprootGroup,
		"mapall_user":      opts.MapallUser,
		"mapall_group":     opts.MapallGroup,
		"security":         nfsStringList(opts.Security),
		"enabled":          opts.Enabled,
		"expose_snapshots": opts.ExposeSnapshots,
	}
}

// nfsStringList normalises a nil slice to an empty slice so the API receives
// `[]` rather than `null`, which it rejects.
func nfsStringList(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// nfsShareFromResponse converts a wire-format response to a user-facing NFSShare.
func nfsShareFromResponse(resp nfsShareResponse) NFSShare {
	return NFSShare{
		ID:              resp.ID,
		Path:            resp.Path,
		Aliases:         nfsStringList(resp.Aliases),
		Comment:         resp.Comment,
		Networks:        nfsStringList(resp.Networks),
		Hosts:           nfsStringList(resp.Hosts),
		ReadOnly:        resp.RO,
		MaprootUser:     resp.MaprootUser,
		MaprootGroup:    resp.MaprootGroup,
		MapallUser:      resp.MapallUser,
		MapallGroup:     resp.MapallGroup,
		Security:        nfsStringList(resp.Security),
		Enabled:         resp.Enabled,
		ExposeSnapshots: resp.ExposeSnapshots,
		Locked:          resp.Locked,
	}
}

// isNFSNotFoundError reports whether err is the API's ENOENT signal for a
// missing instance. Only a provable ENOENT counts: generic prose such as the
// JSON-RPC "Method does not exist" reply must surface as a real error rather
// than being mistaken for a deleted share.
func isNFSNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var rpcErr *client.JSONRPCError
	if errors.As(err, &rpcErr) {
		if rpcErr.Data == nil {
			return false
		}
		return rpcErr.Data.Error == errnoENOENT || strings.Contains(rpcErr.Data.Reason, "[ENOENT]")
	}

	return strings.Contains(err.Error(), "[ENOENT]")
}
