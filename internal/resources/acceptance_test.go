package resources_test

import (
	"context"
	"fmt"
	"os"
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
    private_key          = <<-EOK
%[3]s
    EOK
    host_key_fingerprint = %[4]q
  }
}
`, os.Getenv(envHost), os.Getenv(envAPIKey), os.Getenv(envSSHPrivateKey), os.Getenv(envSSHHostFingerprint), testAccAPIUser())
}

// testAccClient builds a client against the acceptance target so tests can
// verify server-side state directly, independent of Terraform state.
func testAccClient(t *testing.T) client.Client {
	t.Helper()

	sshClient, err := client.NewSSHClient(&client.SSHConfig{
		Host:               os.Getenv(envHost),
		PrivateKey:         os.Getenv(envSSHPrivateKey),
		HostKeyFingerprint: os.Getenv(envSSHHostFingerprint),
	})
	if err != nil {
		t.Fatalf("unable to create SSH client: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := sshClient.Connect(ctx); err != nil {
		t.Fatalf("unable to connect to %s: %s", os.Getenv(envHost), err)
	}

	wsClient, err := client.NewWebSocketClient(client.WebSocketConfig{
		Host:               os.Getenv(envHost),
		Username:           testAccAPIUser(),
		APIKey:             os.Getenv(envAPIKey),
		InsecureSkipVerify: true,
		Fallback:           sshClient,
	})
	if err != nil {
		t.Fatalf("unable to create WebSocket client: %s", err)
	}

	if err := wsClient.Connect(ctx); err != nil {
		t.Fatalf("unable to connect WebSocket client: %s", err)
	}

	t.Cleanup(func() { _ = wsClient.Close() })

	return wsClient
}
