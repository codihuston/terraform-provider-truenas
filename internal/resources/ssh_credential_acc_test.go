package resources_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccSSHKeyPair asks the appliance for a throwaway key pair, so no private
// key has to be committed to the repository. Only the private half is used:
// TrueNAS derives the public half from it.
func testAccSSHKeyPair(t *testing.T) string {
	t.Helper()

	raw, err := testAccClient(t).Call(context.Background(), "keychaincredential.generate_ssh_key_pair", nil)
	if err != nil {
		t.Fatalf("generate SSH key pair: %s", err)
	}

	var keypair struct {
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(raw, &keypair); err != nil {
		t.Fatalf("parse generate_ssh_key_pair response: %s", err)
	}
	return keypair.PrivateKey
}

// testAccSSHCredentialConfig renders a key pair plus a connection that uses it.
// keypairBody and credentialBody are spliced into the respective resources so
// steps can vary their attributes.
func testAccSSHCredentialConfig(privateKey, keypairBody, credentialBody string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "truenas_ssh_keypair" "test" {
  name = "tfacc-ssh-keypair"

  private_key = <<-EOK
%[1]s
  EOK
%[2]s
}

resource "truenas_ssh_credential" "test" {
  name           = "tfacc-ssh-credential"
  private_key_id = truenas_ssh_keypair.test.id
%[3]s
}
`, strings.TrimRight(privateKey, "\n"), keypairBody, credentialBody)
}

// queryKeychainCredential returns the credential with the given ID, or nil when
// absent.
func queryKeychainCredential(t *testing.T, id string) (map[string]any, error) {
	t.Helper()

	raw, err := testAccClient(t).Call(
		context.Background(),
		"keychaincredential.query",
		[]any{[]any{[]any{"id", "=", jsonNumber(id)}}},
	)
	if err != nil {
		return nil, fmt.Errorf("query keychain credential %s: %w", id, err)
	}

	var credentials []map[string]any
	if err := json.Unmarshal(raw, &credentials); err != nil {
		return nil, fmt.Errorf("parse keychain credential query response: %w", err)
	}
	if len(credentials) == 0 {
		return nil, nil
	}
	return credentials[0], nil
}

// keychainCredentialAttributes returns the attributes of the credential the
// named resource points at.
func keychainCredentialAttributes(t *testing.T, s *terraform.State, name string) (map[string]any, *terraform.ResourceState, error) {
	t.Helper()

	rs, ok := s.RootModule().Resources[name]
	if !ok {
		return nil, nil, fmt.Errorf("resource %s not found in state", name)
	}
	if rs.Primary.ID == "" {
		return nil, nil, fmt.Errorf("resource %s has no ID", name)
	}

	credential, err := queryKeychainCredential(t, rs.Primary.ID)
	if err != nil {
		return nil, nil, err
	}
	if credential == nil {
		return nil, nil, fmt.Errorf("keychain credential %s not found on server", rs.Primary.ID)
	}

	attributes, ok := credential["attributes"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("server attributes are %T, want an object", credential["attributes"])
	}
	return attributes, rs, nil
}

// testAccCheckSSHCredentialExists verifies via the API that the connection in
// state exists, is of the right type, and points at the managed key pair.
func testAccCheckSSHCredentialExists(t *testing.T, name, keypairName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributes, rs, err := keychainCredentialAttributes(t, s, name)
		if err != nil {
			return err
		}

		credential, err := queryKeychainCredential(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if got := credential["type"]; got != "SSH_CREDENTIALS" {
			return fmt.Errorf("server type %v, want SSH_CREDENTIALS", got)
		}

		keypair, ok := s.RootModule().Resources[keypairName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", keypairName)
		}
		if got := fmt.Sprint(attributes["private_key"]); got != keypair.Primary.ID {
			return fmt.Errorf("server private_key %v does not reference key pair %s", got, keypair.Primary.ID)
		}
		return nil
	}
}

// testAccCheckSSHCredentialAttr verifies a single connection attribute
// server-side.
func testAccCheckSSHCredentialAttr(t *testing.T, name, key string, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributes, _, err := keychainCredentialAttributes(t, s, name)
		if err != nil {
			return err
		}
		if got := fmt.Sprint(attributes[key]); got != fmt.Sprint(want) {
			return fmt.Errorf("server %s = %v, want %v", key, got, want)
		}
		return nil
	}
}

// testAccCheckSSHCredentialHostKeyScanned verifies the connection was pinned to
// a host key discovered at create time, rather than left without one.
func testAccCheckSSHCredentialHostKeyScanned(t *testing.T, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributes, rs, err := keychainCredentialAttributes(t, s, name)
		if err != nil {
			return err
		}

		hostKey, ok := attributes["remote_host_key"].(string)
		if !ok || strings.TrimSpace(hostKey) == "" {
			return fmt.Errorf("server remote_host_key is empty; the host key was not discovered")
		}
		if got := rs.Primary.Attributes["remote_host_key"]; got != hostKey {
			return fmt.Errorf("state remote_host_key %q does not match the server's %q", got, hostKey)
		}
		return nil
	}
}

// testAccCheckSSHKeypairExists verifies the key pair exists server-side and
// that its public key is the one Terraform recorded.
func testAccCheckSSHKeypairExists(t *testing.T, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributes, rs, err := keychainCredentialAttributes(t, s, name)
		if err != nil {
			return err
		}

		credential, err := queryKeychainCredential(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if got := credential["type"]; got != "SSH_KEY_PAIR" {
			return fmt.Errorf("server type %v, want SSH_KEY_PAIR", got)
		}
		if got := fmt.Sprint(attributes["public_key"]); got != rs.Primary.Attributes["public_key"] {
			return fmt.Errorf("server public_key %v does not match state %q", got, rs.Primary.Attributes["public_key"])
		}
		// The provider writes the private key and never reads it back, so it
		// must not appear in state even though the API returns it.
		if _, ok := rs.Primary.Attributes["private_key"]; ok {
			return fmt.Errorf("private_key leaked into state")
		}
		return nil
	}
}

// testAccCheckSSHKeypairPublicKeyChanged verifies a rotation actually replaced
// the stored key, by comparing the server's public key with a previous one.
func testAccCheckSSHKeypairPublicKeyChanged(t *testing.T, name string, previous *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attributes, _, err := keychainCredentialAttributes(t, s, name)
		if err != nil {
			return err
		}

		publicKey := fmt.Sprint(attributes["public_key"])
		if *previous == "" {
			*previous = publicKey
			return nil
		}
		if publicKey == *previous {
			return fmt.Errorf("server public_key is unchanged after a private_key_wo_version bump")
		}
		return nil
	}
}

// testAccCheckKeychainCredentialDestroy verifies every credential in the plan
// is gone.
func testAccCheckKeychainCredentialDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if rs.Type != "truenas_ssh_credential" && rs.Type != "truenas_ssh_keypair" {
				continue
			}

			credential, err := queryKeychainCredential(t, rs.Primary.ID)
			if err != nil {
				return err
			}
			if credential != nil {
				return fmt.Errorf("keychain credential %s (%s) still exists after destroy", rs.Primary.ID, name)
			}
		}
		return nil
	}
}

func TestAccSSHCredentialResource(t *testing.T) {
	const credentialName = "truenas_ssh_credential.test"
	const keypairName = "truenas_ssh_keypair.test"

	testAccRequireLiveTarget(t)

	// The appliance can reach its own SSH server, so localhost gives a real
	// remote host to scan and connect to without a second machine.
	privateKey := testAccSSHKeyPair(t)
	rotatedKey := testAccSSHKeyPair(t)

	var publicKey string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainCredentialDestroy(t),
		Steps: []resource.TestStep{
			// Create both objects, discovering the host key on the way.
			{
				Config: testAccSSHCredentialConfig(privateKey, `
  private_key_wo_version = 1`, `
  host = "localhost"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSSHKeypairExists(t, keypairName),
					testAccCheckSSHCredentialExists(t, credentialName, keypairName),
					testAccCheckSSHCredentialHostKeyScanned(t, credentialName),
					resource.TestCheckResourceAttrSet(keypairName, "id"),
					resource.TestCheckResourceAttrSet(keypairName, "public_key"),
					resource.TestCheckNoResourceAttr(keypairName, "private_key"),
					resource.TestCheckResourceAttr(credentialName, "host", "localhost"),
					// Unset optional attributes fall back to the API's defaults.
					resource.TestCheckResourceAttr(credentialName, "port", "22"),
					resource.TestCheckResourceAttr(credentialName, "username", "root"),
					resource.TestCheckResourceAttr(credentialName, "connect_timeout", "10"),
					resource.TestCheckResourceAttrPair(credentialName, "private_key_id", keypairName, "id"),
					testAccCheckSSHCredentialAttr(t, credentialName, "host", "localhost"),
					testAccCheckSSHCredentialAttr(t, credentialName, "port", 22),
					testAccCheckSSHCredentialAttr(t, credentialName, "username", "root"),
					testAccCheckSSHCredentialAttr(t, credentialName, "connect_timeout", 10),
					testAccCheckSSHKeypairPublicKeyChanged(t, keypairName, &publicKey),
				),
			},
			// Import the connection by ID. Everything it manages is readable.
			{
				ResourceName:      credentialName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Import the key pair by ID. The private key is write-only, so
			// neither it nor its version survives the round trip.
			{
				ResourceName:            keypairName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"private_key_wo_version"},
			},
			// Update the connection in place, and rotate the key behind it.
			{
				Config: testAccSSHCredentialConfig(rotatedKey, `
  private_key_wo_version = 2`, `
  host            = "127.0.0.1"
  port            = 22
  username        = "truenas_admin"
  connect_timeout = 30`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSSHKeypairExists(t, keypairName),
					testAccCheckSSHCredentialExists(t, credentialName, keypairName),
					resource.TestCheckResourceAttr(credentialName, "host", "127.0.0.1"),
					resource.TestCheckResourceAttr(credentialName, "username", "truenas_admin"),
					resource.TestCheckResourceAttr(credentialName, "connect_timeout", "30"),
					testAccCheckSSHCredentialAttr(t, credentialName, "host", "127.0.0.1"),
					testAccCheckSSHCredentialAttr(t, credentialName, "username", "truenas_admin"),
					testAccCheckSSHCredentialAttr(t, credentialName, "connect_timeout", 30),
					testAccCheckSSHKeypairPublicKeyChanged(t, keypairName, &publicKey),
				),
			},
		},
	})
}

// TestAccSSHCredentialResource_ExplicitHostKey asserts a configured
// remote_host_key is stored verbatim and never overwritten by a scan.
func TestAccSSHCredentialResource_ExplicitHostKey(t *testing.T) {
	const credentialName = "truenas_ssh_credential.test"

	testAccRequireLiveTarget(t)

	privateKey := testAccSSHKeyPair(t)

	raw, err := testAccClient(t).Call(
		context.Background(),
		"keychaincredential.remote_ssh_host_key_scan",
		map[string]any{"host": "localhost"},
	)
	if err != nil {
		t.Fatalf("scan localhost host key: %s", err)
	}

	var hostKey string
	if err := json.Unmarshal(raw, &hostKey); err != nil {
		t.Fatalf("parse remote_ssh_host_key_scan response: %s", err)
	}
	hostKey = strings.TrimRight(hostKey, "\n")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainCredentialDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccSSHCredentialConfig(privateKey, `
  private_key_wo_version = 1`, fmt.Sprintf(`
  host = "localhost"

  remote_host_key = %[1]q`, hostKey)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(credentialName, "remote_host_key", hostKey),
					testAccCheckSSHCredentialAttr(t, credentialName, "remote_host_key", hostKey),
				),
			},
		},
	})
}

// TestAccSSHCredentialResource_UnreachableHost asserts an unreachable host
// fails the apply rather than storing a connection that trusts nothing.
func TestAccSSHCredentialResource_UnreachableHost(t *testing.T) {
	testAccRequireLiveTarget(t)

	privateKey := testAccSSHKeyPair(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainCredentialDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccSSHCredentialConfig(privateKey, `
  private_key_wo_version = 1`, `
  host            = "localhost"
  port            = 1
  connect_timeout = 1`),
				ExpectError: regexp.MustCompile(`Unable to Discover Remote Host Key`),
			},
		},
	})
}
