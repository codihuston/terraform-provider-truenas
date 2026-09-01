package resources_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/deevus/terraform-provider-truenas/internal/resources"
	"github.com/deevus/terraform-provider-truenas/internal/services"
	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeAPIKeyServer is an in-memory stand-in for the api_key.* namespace. It
// reproduces the behaviour the resource is designed around: a secret is
// disclosed only in the reply that creates the key, and every later reply
// carries an empty one.
type fakeAPIKeyServer struct {
	mu     sync.Mutex
	nextID int64
	keys   map[int64]services.APIKey
	// creates counts issued keys, so a test can assert that a step reused a key
	// rather than replacing it.
	creates int
}

func newFakeAPIKeyServer() *fakeAPIKeyServer {
	return &fakeAPIKeyServer{nextID: 1, keys: map[int64]services.APIKey{}}
}

func (f *fakeAPIKeyServer) service() *services.MockAPIKeyService {
	return &services.MockAPIKeyService{
		CreateFunc: func(_ context.Context, opts services.CreateAPIKeyOpts) (*services.APIKey, error) {
			f.mu.Lock()
			defer f.mu.Unlock()

			id := f.nextID
			f.nextID++
			f.creates++

			username := opts.Username
			key := services.APIKey{
				ID:             id,
				Name:           opts.Name,
				Username:       &username,
				UserIdentifier: "3001",
				CreatedAt:      time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
				ExpiresAt:      utcPointer(opts.ExpiresAt),
				Local:          true,
			}
			f.keys[id] = key

			// Only the creating reply carries the secret.
			issued := key
			issued.Key = fmt.Sprintf("%d-%s", id, "amfpg5iQSKI5rClsylOGH09wZnCvcmsKUJdGxu9yUddRkQWewAW21fgLwdowOiVy")
			return &issued, nil
		},
		GetFunc: func(_ context.Context, id int64) (*services.APIKey, error) {
			f.mu.Lock()
			defer f.mu.Unlock()

			key, ok := f.keys[id]
			if !ok {
				return nil, nil
			}
			return &key, nil
		},
		UpdateFunc: func(_ context.Context, id int64, opts services.UpdateAPIKeyOpts) (*services.APIKey, error) {
			f.mu.Lock()
			defer f.mu.Unlock()

			key, ok := f.keys[id]
			if !ok {
				return nil, fmt.Errorf("[ENOENT] api_key_update.id: Entry not found")
			}
			key.Name = opts.Name
			key.ExpiresAt = utcPointer(opts.ExpiresAt)
			f.keys[id] = key
			return &key, nil
		},
		DeleteFunc: func(_ context.Context, id int64) error {
			f.mu.Lock()
			defer f.mu.Unlock()

			delete(f.keys, id)
			return nil
		},
	}
}

// utcPointer mirrors the server, which reports every timestamp back in UTC
// regardless of the offset it was given.
func utcPointer(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

// lifecycleProvider serves only truenas_api_key, wired to a fake server, so the
// real terraform binary can drive the resource without a TrueNAS host.
type lifecycleProvider struct {
	svcs *services.TrueNASServices
}

func (p *lifecycleProvider) Metadata(_ context.Context, _ fwprovider.MetadataRequest, resp *fwprovider.MetadataResponse) {
	resp.TypeName = "truenas"
}

func (p *lifecycleProvider) Schema(_ context.Context, _ fwprovider.SchemaRequest, resp *fwprovider.SchemaResponse) {
	resp.Schema = providerschema.Schema{}
}

func (p *lifecycleProvider) Configure(_ context.Context, _ fwprovider.ConfigureRequest, resp *fwprovider.ConfigureResponse) {
	resp.ResourceData = p.svcs
}

func (p *lifecycleProvider) Resources(_ context.Context) []func() fwresource.Resource {
	return []func() fwresource.Resource{resources.NewAPIKeyResource}
}

func (p *lifecycleProvider) DataSources(_ context.Context) []func() fwdatasource.DataSource {
	return nil
}

func lifecycleProviderFactories(f *fakeAPIKeyServer) map[string]func() (tfprotov6.ProviderServer, error) {
	p := &lifecycleProvider{svcs: &services.TrueNASServices{APIKey: f.service()}}
	return map[string]func() (tfprotov6.ProviderServer, error){
		"truenas": providerserver.NewProtocol6WithError(p),
	}
}

var regexpInvalidTimestamp = regexp.MustCompile(`Invalid RFC 3339 Timestamp`)

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

const fakeAPIKeySecret = "1-amfpg5iQSKI5rClsylOGH09wZnCvcmsKUJdGxu9yUddRkQWewAW21fgLwdowOiVy"

// TestAPIKeyResourceLifecycle drives truenas_api_key through the real terraform
// binary against the fake server: a same-apply consumer reads the secret, a
// rename keeps it, an expiry written with a UTC offset survives the
// consistency check, and destroy removes the key server-side.
func TestAPIKeyResourceLifecycle(t *testing.T) {
	requireTerraformCLI(t)

	f := newFakeAPIKeyServer()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: lifecycleProviderFactories(f),
		CheckDestroy:             checkFakeAPIKeysDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: `
resource "truenas_api_key" "test" {
  name     = "terraform-lifecycle"
  username = "root"
}

resource "terraform_data" "consumer" {
  input = truenas_api_key.test.key
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_api_key.test", "id", "1"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "name", "terraform-lifecycle"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "username", "root"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "store_key", "true"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "key", fakeAPIKeySecret),
					resource.TestCheckResourceAttr("truenas_api_key.test", "created_at", "2026-01-02T03:04:05Z"),
					resource.TestCheckNoResourceAttr("truenas_api_key.test", "expires_at"),
					// A resource later in the same apply read the secret.
					resource.TestCheckResourceAttr("terraform_data.consumer", "output", fakeAPIKeySecret),
					checkFakeAPIKeyName(f, 1, "terraform-lifecycle"),
				),
			},
			{
				// Rename in place, and set an expiry in a non-UTC offset that
				// the server reports back in UTC.
				Config: `
resource "truenas_api_key" "test" {
  name       = "terraform-lifecycle-renamed"
  username   = "root"
  expires_at = "2035-01-02T15:04:05+10:00"
}

resource "terraform_data" "consumer" {
  input = truenas_api_key.test.key
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("truenas_api_key.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_api_key.test", "id", "1"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "name", "terraform-lifecycle-renamed"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "expires_at", "2035-01-02T15:04:05+10:00"),
					// The secret survives an update the API never re-issues it in.
					resource.TestCheckResourceAttr("truenas_api_key.test", "key", fakeAPIKeySecret),
					resource.TestCheckResourceAttr("terraform_data.consumer", "output", fakeAPIKeySecret),
					checkFakeAPIKeyName(f, 1, "terraform-lifecycle-renamed"),
					checkFakeAPIKeyCreates(f, 1),
				),
			},
			{
				// The refreshed state holds the UTC form of the same instant,
				// so the offset the config keeps must not plan a change.
				Config: `
resource "truenas_api_key" "test" {
  name       = "terraform-lifecycle-renamed"
  username   = "root"
  expires_at = "2035-01-02T15:04:05+10:00"
}

resource "terraform_data" "consumer" {
  input = truenas_api_key.test.key
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// Clearing the expiry sends an explicit null.
				Config: `
resource "truenas_api_key" "test" {
  name     = "terraform-lifecycle-renamed"
  username = "root"
}

resource "terraform_data" "consumer" {
  input = truenas_api_key.test.key
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("truenas_api_key.test", "expires_at"),
					checkFakeAPIKeyExpiry(f, 1, nil),
				),
			},
			{
				// An imported key has no secret and no store_key in state; the
				// plan that follows the import must still be empty.
				ResourceName:            "truenas_api_key.test",
				ImportState:             true,
				ImportStateId:           "1",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"},
			},
			{
				// Changing the owning account cannot be done in place.
				Config: `
resource "truenas_api_key" "test" {
  name     = "terraform-lifecycle-renamed"
  username = "someone-else"
}

resource "terraform_data" "consumer" {
  input = truenas_api_key.test.key
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("truenas_api_key.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_api_key.test", "id", "2"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "username", "someone-else"),
					checkFakeAPIKeyCreates(f, 2),
				),
			},
		},
	})
}

// TestAPIKeyResourceStoreKeyLifecycle covers the store_key contract: the secret
// is returned by the creating apply, leaves state at the next refresh, and can
// only come back by issuing a new key.
func TestAPIKeyResourceStoreKeyLifecycle(t *testing.T) {
	requireTerraformCLI(t)

	f := newFakeAPIKeyServer()

	const unstoredConfig = `
resource "truenas_api_key" "test" {
  name      = "terraform-unstored"
  username  = "root"
  store_key = false
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: lifecycleProviderFactories(f),
		CheckDestroy:             checkFakeAPIKeysDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: unstoredConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_api_key.test", "store_key", "false"),
					// The creating apply still hands out the secret.
					resource.TestCheckResourceAttr("truenas_api_key.test", "key", fakeAPIKeySecret),
				),
			},
			{
				// The refresh at the head of this step scrubs the secret, and
				// the config is unchanged, so nothing is planned.
				Config: unstoredConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("truenas_api_key.test", "key"),
					checkFakeAPIKeyCreates(f, 1),
				),
			},
			{
				// Storing the key again is only possible by issuing a new one.
				Config: `
resource "truenas_api_key" "test" {
  name      = "terraform-unstored"
  username  = "root"
  store_key = true
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("truenas_api_key.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_api_key.test", "id", "2"),
					resource.TestCheckResourceAttr("truenas_api_key.test", "key", "2-amfpg5iQSKI5rClsylOGH09wZnCvcmsKUJdGxu9yUddRkQWewAW21fgLwdowOiVy"),
					checkFakeAPIKeyCreates(f, 2),
				),
			},
		},
	})
}

// TestAPIKeyResourceInvalidExpiry checks that a malformed expiry is rejected at
// plan time, before anything reaches the server.
func TestAPIKeyResourceInvalidExpiry(t *testing.T) {
	requireTerraformCLI(t)

	f := newFakeAPIKeyServer()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: lifecycleProviderFactories(f),
		Steps: []resource.TestStep{
			{
				Config: `
resource "truenas_api_key" "test" {
  name       = "terraform-bad-expiry"
  username   = "root"
  expires_at = "not-a-timestamp"
}
`,
				ExpectError: regexpInvalidTimestamp,
				Check: func(*terraform.State) error {
					return checkFakeAPIKeyCreates(f, 0)(nil)
				},
			},
		},
	})
}

func checkFakeAPIKeyName(f *fakeAPIKeyServer, id int64, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()

		key, ok := f.keys[id]
		if !ok {
			return fmt.Errorf("API key %d not found on the server", id)
		}
		if key.Name != want {
			return fmt.Errorf("server name is %q, want %q", key.Name, want)
		}
		return nil
	}
}

func checkFakeAPIKeyExpiry(f *fakeAPIKeyServer, id int64, want *time.Time) resource.TestCheckFunc {
	return func(*terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()

		key, ok := f.keys[id]
		if !ok {
			return fmt.Errorf("API key %d not found on the server", id)
		}
		switch {
		case want == nil && key.ExpiresAt != nil:
			return fmt.Errorf("server expiry is %s, want none", key.ExpiresAt)
		case want != nil && (key.ExpiresAt == nil || !key.ExpiresAt.Equal(*want)):
			return fmt.Errorf("server expiry is %v, want %s", key.ExpiresAt, want)
		}
		return nil
	}
}

func checkFakeAPIKeyCreates(f *fakeAPIKeyServer, want int) resource.TestCheckFunc {
	return func(*terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.creates != want {
			return fmt.Errorf("server issued %d keys, want %d", f.creates, want)
		}
		return nil
	}
}

func checkFakeAPIKeysDestroyed(f *fakeAPIKeyServer) resource.TestCheckFunc {
	return func(*terraform.State) error {
		f.mu.Lock()
		defer f.mu.Unlock()

		if len(f.keys) != 0 {
			return fmt.Errorf("%d API keys remain on the server after destroy", len(f.keys))
		}
		return nil
	}
}
