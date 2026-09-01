package resources_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"sync"
	"testing"

	"github.com/deevus/terraform-provider-truenas/internal/resources"
	"github.com/deevus/terraform-provider-truenas/internal/services"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// Diagnostics the resources raise, matched as a user would see them.
var (
	regexpUnableToDiscoverHostKey = regexp.MustCompile(`Unable to Discover Remote Host Key`)
	regexpInvalidAttributeValue   = regexp.MustCompile(`Invalid Attribute Value`)
)

// fakeKeychain is an in-memory stand-in for the keychaincredential.* namespace.
// It lets the whole Terraform lifecycle — plan, apply, import, update,
// destroy — run against the resources without a TrueNAS appliance, and records
// what the provider actually sent so server-side effects can be asserted the
// way the acceptance tests assert them against a live server.
type fakeKeychain struct {
	mu          sync.Mutex
	nextID      int64
	keypairs    map[int64]*services.SSHKeyPair
	privateKeys map[int64]string
	credentials map[int64]*services.SSHCredential
	scans       []services.ScanRemoteHostKeyOpts
}

func newFakeKeychain() *fakeKeychain {
	return &fakeKeychain{
		nextID:      100,
		keypairs:    map[int64]*services.SSHKeyPair{},
		privateKeys: map[int64]string{},
		credentials: map[int64]*services.SSHCredential{},
	}
}

// publicKeyFor stands in for the appliance deriving the public half from the
// private one: a different private key yields a different public key.
func publicKeyFor(privateKey string) string {
	sum := sha256.Sum256([]byte(privateKey))
	return "ssh-ed25519 " + hex.EncodeToString(sum[:8])
}

// hostKeyFor stands in for a host key scan: reachable hosts answer with a key
// that identifies them, port 1 is unreachable.
func hostKeyFor(opts services.ScanRemoteHostKeyOpts) (string, error) {
	if opts.Port == 1 {
		return "", fmt.Errorf("connection refused")
	}
	return fmt.Sprintf("%s:%d ssh-ed25519 AAAAHOSTKEY", opts.Host, opts.Port), nil
}

func (f *fakeKeychain) CreateSSHKeyPair(_ context.Context, opts services.CreateSSHKeyPairOpts) (*services.SSHKeyPair, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	keypair := &services.SSHKeyPair{ID: f.nextID, Name: opts.Name, PublicKey: publicKeyFor(opts.PrivateKey)}
	f.keypairs[keypair.ID] = keypair
	f.privateKeys[keypair.ID] = opts.PrivateKey
	return keypair, nil
}

func (f *fakeKeychain) GetSSHKeyPair(_ context.Context, id int64) (*services.SSHKeyPair, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	keypair, ok := f.keypairs[id]
	if !ok {
		return nil, nil
	}
	copied := *keypair
	return &copied, nil
}

func (f *fakeKeychain) UpdateSSHKeyPair(_ context.Context, id int64, opts services.UpdateSSHKeyPairOpts) (*services.SSHKeyPair, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	keypair, ok := f.keypairs[id]
	if !ok {
		return nil, fmt.Errorf("keychain credential %d not found", id)
	}
	keypair.Name = opts.Name
	if opts.PrivateKey != nil {
		keypair.PublicKey = publicKeyFor(*opts.PrivateKey)
		f.privateKeys[id] = *opts.PrivateKey
	}
	copied := *keypair
	return &copied, nil
}

func (f *fakeKeychain) CreateSSHCredential(_ context.Context, opts services.CreateSSHCredentialOpts) (*services.SSHCredential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	credential := &services.SSHCredential{
		ID:             f.nextID,
		Name:           opts.Name,
		Host:           opts.Host,
		Port:           opts.Port,
		Username:       opts.Username,
		PrivateKeyID:   opts.PrivateKeyID,
		RemoteHostKey:  opts.RemoteHostKey,
		ConnectTimeout: opts.ConnectTimeout,
	}
	f.credentials[credential.ID] = credential
	return credential, nil
}

func (f *fakeKeychain) GetSSHCredential(_ context.Context, id int64) (*services.SSHCredential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	credential, ok := f.credentials[id]
	if !ok {
		return nil, nil
	}
	copied := *credential
	return &copied, nil
}

func (f *fakeKeychain) UpdateSSHCredential(ctx context.Context, id int64, opts services.UpdateSSHCredentialOpts) (*services.SSHCredential, error) {
	f.mu.Lock()
	if _, ok := f.credentials[id]; !ok {
		f.mu.Unlock()
		return nil, fmt.Errorf("keychain credential %d not found", id)
	}
	f.mu.Unlock()

	updated, err := f.CreateSSHCredential(ctx, opts)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.credentials, updated.ID)
	updated.ID = id
	f.credentials[id] = updated
	copied := *updated
	return &copied, nil
}

func (f *fakeKeychain) Delete(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.keypairs, id)
	delete(f.privateKeys, id)
	delete(f.credentials, id)
	return nil
}

func (f *fakeKeychain) ScanRemoteHostKey(_ context.Context, opts services.ScanRemoteHostKeyOpts) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.scans = append(f.scans, opts)
	return hostKeyFor(opts)
}

func (f *fakeKeychain) scanCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.scans)
}

func (f *fakeKeychain) storedPrivateKey(id int64) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.privateKeys[id]
}

func (f *fakeKeychain) objectCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.keypairs) + len(f.credentials)
}

// fakeProvider serves the SSH keychain resources backed by fakeKeychain.
type fakeProvider struct {
	keychain *fakeKeychain
}

func (p *fakeProvider) Metadata(_ context.Context, _ fwprovider.MetadataRequest, resp *fwprovider.MetadataResponse) {
	resp.TypeName = "truenas"
}

func (p *fakeProvider) Schema(_ context.Context, _ fwprovider.SchemaRequest, resp *fwprovider.SchemaResponse) {
	resp.Schema = providerschema.Schema{}
}

func (p *fakeProvider) Configure(_ context.Context, _ fwprovider.ConfigureRequest, resp *fwprovider.ConfigureResponse) {
	svc := &services.TrueNASServices{KeychainCredential: p.keychain}
	resp.DataSourceData = svc
	resp.ResourceData = svc
}

func (p *fakeProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

func (p *fakeProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewSSHKeypairResource,
		resources.NewSSHCredentialResource,
	}
}

// fakeProviderFactories serves the fake provider under the same address the
// real one uses, so configurations are written exactly as a user would.
func fakeProviderFactories(keychain *fakeKeychain) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"truenas": providerserver.NewProtocol6WithError(&fakeProvider{keychain: keychain}),
	}
}

// requireTerraformCLI skips when no Terraform binary is available, so the unit
// suite keeps running on machines without one.
func requireTerraformCLI(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform CLI not available")
	}
}

func sshLifecycleConfig(keypairBody, credentialBody string) string {
	return fmt.Sprintf(`
resource "truenas_ssh_keypair" "test" {
  name = "lifecycle-keypair"
%[1]s
}

resource "truenas_ssh_credential" "test" {
  name           = "lifecycle-credential"
  private_key_id = truenas_ssh_keypair.test.id
%[2]s
}
`, keypairBody, credentialBody)
}

// checkStoredPrivateKey asserts the appliance holds the key the configuration
// supplied, even though Terraform never reads it back.
func checkStoredPrivateKey(keychain *fakeKeychain, name, want string) testresource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		var id int64
		if _, err := fmt.Sscan(rs.Primary.ID, &id); err != nil {
			return fmt.Errorf("resource %s has a non-numeric ID %q", name, rs.Primary.ID)
		}
		if got := keychain.storedPrivateKey(id); got != want {
			return fmt.Errorf("server private key for %s is %q, want %q", name, got, want)
		}
		return nil
	}
}

// TestSSHCredentialLifecycle drives the full Terraform lifecycle against an
// in-memory keychain: create with a scanned host key, import both objects,
// rotate the key in place, and destroy. It covers what the TF_ACC tests cover
// on a live appliance, minus the appliance.
func TestSSHCredentialLifecycle(t *testing.T) {
	requireTerraformCLI(t)

	const credentialName = "truenas_ssh_credential.test"
	const keypairName = "truenas_ssh_keypair.test"
	const originalKey = "-----BEGIN PRIVATE KEY-----\noriginal\n-----END PRIVATE KEY-----"
	const rotatedKey = "-----BEGIN PRIVATE KEY-----\nrotated\n-----END PRIVATE KEY-----"

	keychain := newFakeKeychain()

	testresource.UnitTest(t, testresource.TestCase{
		// Write-only attributes require Terraform 1.11.
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		ProtoV6ProviderFactories: fakeProviderFactories(keychain),
		CheckDestroy: func(*terraform.State) error {
			if got := keychain.objectCount(); got != 0 {
				return fmt.Errorf("%d keychain objects survived destroy", got)
			}
			return nil
		},
		Steps: []testresource.TestStep{
			// Create both objects. The host key is discovered on the way and
			// the private key never lands in state.
			{
				Config: sshLifecycleConfig(fmt.Sprintf(`
  private_key            = %[1]q
  private_key_wo_version = 1`, originalKey), `
  host = "backup.example.com"`),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckNoResourceAttr(keypairName, "private_key"),
					testresource.TestCheckResourceAttr(keypairName, "public_key", publicKeyFor(originalKey)),
					testresource.TestCheckResourceAttrPair(credentialName, "private_key_id", keypairName, "id"),
					testresource.TestCheckResourceAttr(credentialName, "host", "backup.example.com"),
					testresource.TestCheckResourceAttr(credentialName, "port", "22"),
					testresource.TestCheckResourceAttr(credentialName, "username", "root"),
					testresource.TestCheckResourceAttr(credentialName, "connect_timeout", "10"),
					testresource.TestCheckResourceAttr(credentialName, "remote_host_key", "backup.example.com:22 ssh-ed25519 AAAAHOSTKEY"),
					checkStoredPrivateKey(keychain, keypairName, originalKey),
				),
			},
			// A second plan is empty: the host key is not re-scanned, so
			// nothing drifts.
			{
				Config: sshLifecycleConfig(fmt.Sprintf(`
  private_key            = %[1]q
  private_key_wo_version = 1`, originalKey), `
  host = "backup.example.com"`),
				PlanOnly: true,
				Check: func(*terraform.State) error {
					if got := keychain.scanCount(); got != 1 {
						return fmt.Errorf("host key was scanned %d times, want 1", got)
					}
					return nil
				},
			},
			// Import the connection by ID.
			{
				ResourceName:      credentialName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Import the key pair by ID. The write-only key and its version do
			// not survive the round trip.
			{
				ResourceName:            keypairName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"private_key_wo_version"},
			},
			// Update in place: rotate the key and move the connection.
			{
				Config: sshLifecycleConfig(fmt.Sprintf(`
  private_key            = %[1]q
  private_key_wo_version = 2`, rotatedKey), `
  host            = "moved.example.com"
  port            = 2222
  username        = "truenas_replication"
  connect_timeout = 30`),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttr(keypairName, "public_key", publicKeyFor(rotatedKey)),
					testresource.TestCheckResourceAttr(credentialName, "host", "moved.example.com"),
					testresource.TestCheckResourceAttr(credentialName, "port", "2222"),
					testresource.TestCheckResourceAttr(credentialName, "username", "truenas_replication"),
					testresource.TestCheckResourceAttr(credentialName, "connect_timeout", "30"),
					// The connection followed the host, so it trusts the new
					// host's key.
					testresource.TestCheckResourceAttr(credentialName, "remote_host_key", "moved.example.com:2222 ssh-ed25519 AAAAHOSTKEY"),
					checkStoredPrivateKey(keychain, keypairName, rotatedKey),
				),
			},
		},
	})
}

// TestSSHCredentialLifecycle_ExplicitHostKey asserts a pinned host key is
// stored verbatim and no scan happens.
func TestSSHCredentialLifecycle_ExplicitHostKey(t *testing.T) {
	requireTerraformCLI(t)

	const credentialName = "truenas_ssh_credential.test"
	const hostKey = "backup.example.com ssh-ed25519 AAAAPINNED"

	keychain := newFakeKeychain()

	testresource.UnitTest(t, testresource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		ProtoV6ProviderFactories: fakeProviderFactories(keychain),
		Steps: []testresource.TestStep{
			{
				Config: sshLifecycleConfig(`
  private_key            = "key"
  private_key_wo_version = 1`, fmt.Sprintf(`
  host            = "backup.example.com"
  remote_host_key = %[1]q`, hostKey)),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttr(credentialName, "remote_host_key", hostKey),
					func(*terraform.State) error {
						if got := keychain.scanCount(); got != 0 {
							return fmt.Errorf("host key was scanned %d times despite being pinned", got)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestSSHCredentialLifecycle_UnreachableHost asserts an unreachable host fails
// the apply instead of storing a connection that trusts nothing.
func TestSSHCredentialLifecycle_UnreachableHost(t *testing.T) {
	requireTerraformCLI(t)

	keychain := newFakeKeychain()

	testresource.UnitTest(t, testresource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		ProtoV6ProviderFactories: fakeProviderFactories(keychain),
		Steps: []testresource.TestStep{
			{
				Config: sshLifecycleConfig(`
  private_key            = "key"
  private_key_wo_version = 1`, `
  host = "backup.example.com"
  port = 1`),
				ExpectError: regexpUnableToDiscoverHostKey,
			},
		},
	})
}

// TestSSHCredentialLifecycle_EmptyPrivateKeyRejected asserts the empty key is
// caught at plan time rather than by the appliance.
func TestSSHCredentialLifecycle_EmptyPrivateKeyRejected(t *testing.T) {
	requireTerraformCLI(t)

	keychain := newFakeKeychain()

	testresource.UnitTest(t, testresource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		ProtoV6ProviderFactories: fakeProviderFactories(keychain),
		Steps: []testresource.TestStep{
			{
				Config: `
resource "truenas_ssh_keypair" "test" {
  name                   = "lifecycle-keypair"
  private_key            = ""
  private_key_wo_version = 1
}
`,
				ExpectError: regexpInvalidAttributeValue,
			},
		},
	})
}

// Compile-time check: the fake honours the same contract as the real service.
var _ services.KeychainCredentialServiceAPI = (*fakeKeychain)(nil)

// Compile-time check: the fake provider is a provider, like the real one.
var _ fwprovider.Provider = (*fakeProvider)(nil)
