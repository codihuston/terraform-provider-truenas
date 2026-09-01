package resources_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/deevus/terraform-provider-truenas/internal/provider"
	"github.com/deevus/truenas-go/client"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Environment variables that configure the acceptance-test target.
const (
	envHost               = "TRUENAS_HOST"
	envAPIKey             = "TRUENAS_API_KEY"
	envAPIUser            = "TRUENAS_API_USER"
	envSSHUser            = "TRUENAS_SSH_USER"
	envSSHPrivateKey      = "TRUENAS_SSH_PRIVATE_KEY"
	envSSHHostFingerprint = "TRUENAS_SSH_HOST_KEY_FINGERPRINT"
	envPool               = "TRUENAS_ACC_POOL"
)

// testAccProtoV6ProviderFactories wires the in-process provider into the
// terraform-plugin-testing harness.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"truenas": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// testAccPreCheck skips the test unless every variable needed to reach a live
// TrueNAS server is set. TF_ACC itself is enforced by resource.Test.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	for _, name := range []string{envHost, envAPIKey, envSSHPrivateKey, envSSHHostFingerprint} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s must be set for acceptance tests", name)
		}
	}
}

// testAccAPIUser returns the user the API key belongs to.
func testAccAPIUser() string {
	if user := os.Getenv(envAPIUser); user != "" {
		return user
	}
	return "root"
}

// testAccSSHUser returns the user the SSH transport authenticates as. The
// provider defaults to root, which not every appliance permits.
func testAccSSHUser() string {
	if user := os.Getenv(envSSHUser); user != "" {
		return user
	}
	return "root"
}

// testAccPool returns the pool acceptance-test resources are created in.
func testAccPool() string {
	if pool := os.Getenv(envPool); pool != "" {
		return pool
	}
	return "tank"
}

// testAccProviderConfig renders a provider block pointing at the acceptance
// target. The private key is passed inline so no file paths leak into state.
func testAccProviderConfig() string {
	return fmt.Sprintf(`
provider "truenas" {
  host        = %[1]q
  auth_method = "websocket"

  websocket {
    username             = %[5]q
    api_key              = %[2]q
    insecure_skip_verify = true
  }

  ssh {
    user                 = %[6]q
    private_key          = <<-EOK
%[3]s
    EOK
    host_key_fingerprint = %[4]q
  }
}
`, os.Getenv(envHost), os.Getenv(envAPIKey), os.Getenv(envSSHPrivateKey), os.Getenv(envSSHHostFingerprint), testAccAPIUser(), testAccSSHUser())
}

// The acceptance target is dialled at most once per test binary: every
// server-side check shares one connection instead of opening an SSH session
// each time.
var (
	testAccClientOnce sync.Once
	testAccSSHClient  *client.SSHClient
	testAccWSClient   *client.WebSocketClient
	testAccClientErr  error
)

// TestMain closes the shared acceptance client, if one was ever created, after
// all tests have run. Nothing connects here, so a run with TF_ACC unset never
// reaches the server.
func TestMain(m *testing.M) {
	code := m.Run()

	if testAccWSClient != nil {
		_ = testAccWSClient.Close()
	}
	if testAccSSHClient != nil {
		_ = testAccSSHClient.Close()
	}

	os.Exit(code)
}

// testAccClient returns the shared client for the acceptance target so tests
// can verify server-side state directly, independent of Terraform state.
func testAccClient(t *testing.T) client.Client {
	t.Helper()

	testAccClientOnce.Do(dialTestAccClient)

	if testAccClientErr != nil {
		t.Fatalf("unable to reach acceptance target: %s", testAccClientErr)
	}
	return testAccWSClient
}

// dialTestAccClient establishes the SSH and WebSocket connections once,
// retaining both so TestMain can close them.
func dialTestAccClient() {
	sshClient, err := client.NewSSHClient(&client.SSHConfig{
		Host:               os.Getenv(envHost),
		User:               testAccSSHUser(),
		PrivateKey:         os.Getenv(envSSHPrivateKey),
		HostKeyFingerprint: os.Getenv(envSSHHostFingerprint),
	})
	if err != nil {
		testAccClientErr = fmt.Errorf("create SSH client: %w", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := sshClient.Connect(ctx); err != nil {
		testAccClientErr = fmt.Errorf("connect to %s: %w", os.Getenv(envHost), err)
		return
	}
	testAccSSHClient = sshClient

	wsClient, err := client.NewWebSocketClient(client.WebSocketConfig{
		Host:               os.Getenv(envHost),
		Username:           testAccAPIUser(),
		APIKey:             os.Getenv(envAPIKey),
		InsecureSkipVerify: true,
		Fallback:           sshClient,
	})
	if err != nil {
		testAccClientErr = fmt.Errorf("create WebSocket client: %w", err)
		return
	}

	if err := wsClient.Connect(ctx); err != nil {
		testAccClientErr = fmt.Errorf("connect WebSocket client: %w", err)
		return
	}
	testAccWSClient = wsClient
}
