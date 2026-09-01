package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"

	truenas "github.com/deevus/truenas-go"
	"github.com/deevus/truenas-go/client"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeAppliance is an in-memory stand-in for the `service` API namespace of a
// TrueNAS appliance. It lets the full Terraform lifecycle run against the real
// provider server without a live box: state changes only through the same
// service.start/stop/update calls the appliance would apply.
type fakeAppliance struct {
	mu       sync.Mutex
	services map[string]*fakeService
	calls    []string
}

type fakeService struct {
	id     int64
	enable bool
	state  string
}

func newFakeAppliance() *fakeAppliance {
	return &fakeAppliance{
		services: map[string]*fakeService{
			"nfs":  {id: 1, enable: false, state: "STOPPED"},
			"cifs": {id: 2, enable: false, state: "STOPPED"},
			"ssh":  {id: 3, enable: true, state: "RUNNING"},
		},
	}
}

func (a *fakeAppliance) snapshot(name string) (enable, running bool, found bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	svc, ok := a.services[name]
	if !ok {
		return false, false, false
	}
	return svc.enable, svc.state == "RUNNING", true
}

// transcript returns the API calls the provider made, in order.
func (a *fakeAppliance) transcript() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]string(nil), a.calls...)
}

func (a *fakeAppliance) call(_ context.Context, method string, params any) (json.RawMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	args, _ := params.([]any)

	switch method {
	case "service.query":
		return a.query(args)
	case "service.update":
		return a.update(args)
	case "service.start", "service.stop":
		return a.control(method, args)
	default:
		return nil, fmt.Errorf("unexpected method %q", method)
	}
}

func (a *fakeAppliance) query(args []any) (json.RawMessage, error) {
	name := queryFilterName(args)
	a.calls = append(a.calls, fmt.Sprintf("service.query %s", orAll(name)))

	out := []map[string]any{}
	for svcName, svc := range a.services {
		if name != "" && svcName != name {
			continue
		}
		out = append(out, map[string]any{
			"id":      svc.id,
			"service": svcName,
			"enable":  svc.enable,
			"state":   svc.state,
		})
	}
	return json.Marshal(out)
}

func (a *fakeAppliance) update(args []any) (json.RawMessage, error) {
	name, _ := args[0].(string)
	payload, _ := args[1].(map[string]any)

	svc, ok := a.services[name]
	if !ok {
		return nil, fmt.Errorf("[ENOENT] service %q not found", name)
	}

	enable, _ := payload["enable"].(bool)
	svc.enable = enable
	a.calls = append(a.calls, fmt.Sprintf("service.update %s enable=%t", name, enable))

	return json.Marshal(svc.id)
}

func (a *fakeAppliance) control(method string, args []any) (json.RawMessage, error) {
	name, _ := args[0].(string)
	opts, _ := args[1].(map[string]any)

	svc, ok := a.services[name]
	if !ok {
		return nil, fmt.Errorf("[ENOENT] service %q not found", name)
	}

	if silent, _ := opts["silent"].(bool); silent {
		return nil, fmt.Errorf("test appliance requires silent=false so failures are explained")
	}

	if method == "service.start" {
		svc.state = "RUNNING"
	} else {
		svc.state = "STOPPED"
	}
	a.calls = append(a.calls, fmt.Sprintf("%s %s (silent=false) -> %s", method, name, svc.state))

	return json.Marshal(true)
}

// queryFilterName extracts the service name from a `service.query` filter of
// the form [["service", "=", "nfs"]], returning "" for an unfiltered query.
func queryFilterName(args []any) string {
	if len(args) == 0 {
		return ""
	}
	filters, ok := args[0].([]any)
	if !ok || len(filters) == 0 {
		return ""
	}
	filter, ok := filters[0].([]any)
	if !ok || len(filter) != 3 {
		return ""
	}
	name, _ := filter[2].(string)
	return name
}

func orAll(name string) string {
	if name == "" {
		return "(all services)"
	}
	return name
}

// testAccFakeProviderFactories serves the real provider with its client
// replaced by the fake appliance.
func testAccFakeProviderFactories(a *fakeAppliance) map[string]func() (tfprotov6.ProviderServer, error) {
	mock := &client.MockClient{
		VersionVal:  truenas.Version{Major: 25, Minor: 10, Patch: 6, Raw: "TrueNAS-SCALE-25.10.6"},
		ConnectFunc: func(ctx context.Context) error { return nil },
		CallFunc:    a.call,
	}

	p := &TrueNASProvider{
		version: "test",
		factory: &mockClientFactory{sshClient: mock, wsClient: mock},
	}

	return map[string]func() (tfprotov6.ProviderServer, error){
		"truenas": providerserver.NewProtocol6WithError(p),
	}
}

const testFakeProviderConfig = `
provider "truenas" {
  host        = "truenas.test"
  auth_method = "websocket"

  websocket {
    username = "root"
    api_key  = "test-key"
  }

  ssh {
    private_key          = "test-key"
    host_key_fingerprint = "SHA256:test"
  }
}
`

// requireTerraformCLI points the harness at the local terraform binary, since
// these tests drive the real CLI rather than a live appliance.
func requireTerraformCLI(t *testing.T) {
	t.Helper()

	if os.Getenv("TF_ACC_TERRAFORM_PATH") != "" {
		return
	}
	path, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform CLI not available")
	}
	t.Setenv("TF_ACC_TERRAFORM_PATH", path)
}

// TestServiceResourceLifecycle drives truenas_service through a real Terraform
// plan/apply/refresh/import/destroy cycle against the fake appliance and
// asserts appliance-side state after every step.
func TestServiceResourceLifecycle(t *testing.T) {
	requireTerraformCLI(t)

	appliance := newFakeAppliance()

	checkAppliance := func(name string, enable, running bool) resource.TestCheckFunc {
		return func(*terraform.State) error {
			gotEnable, gotRunning, ok := appliance.snapshot(name)
			if !ok {
				return fmt.Errorf("service %q missing from appliance", name)
			}
			if gotEnable != enable || gotRunning != running {
				return fmt.Errorf("appliance %s: enable=%t running=%t, want enable=%t running=%t",
					name, gotEnable, gotRunning, enable, running)
			}
			return nil
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccFakeProviderFactories(appliance),
		CheckDestroy: func(*terraform.State) error {
			// Destroy is stop-and-disable: the service still exists.
			enable, running, ok := appliance.snapshot("nfs")
			if !ok {
				return fmt.Errorf("service nfs disappeared from appliance")
			}
			if enable || running {
				return fmt.Errorf("after destroy nfs is enable=%t running=%t, want both false", enable, running)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				// Bare resource: defaults enable and start the service.
				Config: testFakeProviderConfig + `
resource "truenas_service" "nfs" {
  name = "nfs"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.nfs", "id", "nfs"),
					resource.TestCheckResourceAttr("truenas_service.nfs", "name", "nfs"),
					resource.TestCheckResourceAttr("truenas_service.nfs", "enable", "true"),
					resource.TestCheckResourceAttr("truenas_service.nfs", "running", "true"),
					checkAppliance("nfs", true, true),
				),
			},
			{
				// Import by service name.
				ResourceName:      "truenas_service.nfs",
				ImportState:       true,
				ImportStateId:     "nfs",
				ImportStateVerify: true,
			},
			{
				// Stop but keep enabled at boot.
				Config: testFakeProviderConfig + `
resource "truenas_service" "nfs" {
  name    = "nfs"
  enable  = true
  running = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.nfs", "running", "false"),
					checkAppliance("nfs", true, false),
				),
			},
			{
				// Running without start-on-boot.
				Config: testFakeProviderConfig + `
resource "truenas_service" "nfs" {
  name    = "nfs"
  enable  = false
  running = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.nfs", "enable", "false"),
					checkAppliance("nfs", false, true),
				),
			},
			{
				// Out-of-band stop shows up as drift and is corrected.
				PreConfig: func() {
					appliance.mu.Lock()
					appliance.services["nfs"].state = "STOPPED"
					appliance.mu.Unlock()
				},
				Config: testFakeProviderConfig + `
resource "truenas_service" "nfs" {
  name    = "nfs"
  enable  = false
  running = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.nfs", "running", "true"),
					checkAppliance("nfs", false, true),
				),
			},
			{
				// Changing the name replaces the resource, which releases the
				// old service and takes over the new one.
				Config: testFakeProviderConfig + `
resource "truenas_service" "nfs" {
  name = "cifs"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.nfs", "id", "cifs"),
					checkAppliance("cifs", true, true),
					checkAppliance("nfs", false, false),
				),
			},
			{
				// Back to nfs so CheckDestroy asserts on it.
				Config: testFakeProviderConfig + `
resource "truenas_service" "nfs" {
  name = "nfs"
}
`,
				Check: checkAppliance("nfs", true, true),
			},
		},
	})

	t.Logf("appliance API transcript:\n%s", strings.Join(appliance.transcript(), "\n"))
}

// TestServiceResourceUnknownName checks that an unknown service fails at apply
// time with the list of services the appliance actually offers.
func TestServiceResourceUnknownName(t *testing.T) {
	requireTerraformCLI(t)

	appliance := newFakeAppliance()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccFakeProviderFactories(appliance),
		Steps: []resource.TestStep{
			{
				Config: testFakeProviderConfig + `
resource "truenas_service" "bogus" {
  name = "nfsd"
}
`,
				ExpectError: regexp.MustCompile(`TrueNAS does not offer a service named "nfsd"[\s\S]*Known services: cifs, nfs,\s+ssh`),
			},
		},
	})
}
