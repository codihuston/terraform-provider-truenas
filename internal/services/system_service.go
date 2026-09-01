package services

import (
	"context"
	"encoding/json"
	"fmt"

	truenas "github.com/deevus/truenas-go"
)

// ServiceStateRunning is the `state` value the API reports for a running service.
const ServiceStateRunning = "RUNNING"

// SystemService is the user-facing representation of a TrueNAS system service
// (the `service` API namespace). Services are defined by the appliance itself
// and can only be started, stopped and enabled — never created or deleted.
type SystemService struct {
	ID     int64
	Name   string
	Enable bool
	State  string
}

// Running reports whether the service is currently running.
func (s SystemService) Running() bool {
	return s.State == ServiceStateRunning
}

// systemServiceResponse is the wire format returned by the service.* methods.
type systemServiceResponse struct {
	ID      int64  `json:"id"`
	Service string `json:"service"`
	Enable  bool   `json:"enable"`
	State   string `json:"state"`
}

// SystemServices provides typed methods for the service.* API namespace.
type SystemServices struct {
	client truenas.Caller
}

// NewSystemServices creates a new SystemServices.
func NewSystemServices(c truenas.Caller) *SystemServices {
	return &SystemServices{client: c}
}

// Get returns the service with the given name, or nil when the appliance does
// not offer it. Unlike most namespaces, `service.query` reports no match for an
// unknown name rather than raising, so no ENOENT handling is needed.
func (s *SystemServices) Get(ctx context.Context, name string) (*SystemService, error) {
	result, err := s.client.Call(ctx, "service.query", []any{[]any{[]any{"service", "=", name}}})
	if err != nil {
		return nil, err
	}

	var responses []systemServiceResponse
	if err := json.Unmarshal(result, &responses); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}
	if len(responses) == 0 {
		return nil, nil
	}

	svc := systemServiceFromResponse(responses[0])
	return &svc, nil
}

// List returns every service the appliance offers.
func (s *SystemServices) List(ctx context.Context) ([]SystemService, error) {
	result, err := s.client.Call(ctx, "service.query", nil)
	if err != nil {
		return nil, err
	}

	var responses []systemServiceResponse
	if err := json.Unmarshal(result, &responses); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}

	svcs := make([]SystemService, len(responses))
	for i, resp := range responses {
		svcs[i] = systemServiceFromResponse(resp)
	}
	return svcs, nil
}

// SetEnable sets whether the service starts on boot.
func (s *SystemServices) SetEnable(ctx context.Context, name string, enable bool) error {
	_, err := s.client.Call(ctx, "service.update", []any{name, map[string]any{"enable": enable}})
	return err
}

// Start starts the service and waits for it to come up.
func (s *SystemServices) Start(ctx context.Context, name string) error {
	_, err := s.client.Call(ctx, "service.start", []any{name, serviceControlOptions()})
	return err
}

// Stop stops the service and waits for it to go down.
func (s *SystemServices) Stop(ctx context.Context, name string) error {
	_, err := s.client.Call(ctx, "service.stop", []any{name, serviceControlOptions()})
	return err
}

// serviceControlOptions builds the options shared by service.start and
// service.stop. `silent` defaults to true server-side, which turns a failure
// into a `false` return with no explanation; opting out surfaces the reason.
func serviceControlOptions() map[string]any {
	return map[string]any{"silent": false}
}

// systemServiceFromResponse converts a wire-format response to a user-facing
// SystemService.
func systemServiceFromResponse(resp systemServiceResponse) SystemService {
	return SystemService{
		ID:     resp.ID,
		Name:   resp.Service,
		Enable: resp.Enable,
		State:  resp.State,
	}
}
