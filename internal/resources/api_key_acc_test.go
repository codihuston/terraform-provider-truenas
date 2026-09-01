package resources_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccAPIKeyConfig renders an API key owned by the acceptance user. extra is
// spliced into the resource body so steps can vary optional attributes.
func testAccAPIKeyConfig(name, extra string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "truenas_api_key" "test" {
  name     = %[1]q
  username = %[2]q
%[3]s
}
`, name, testAccAPIUser(), extra)
}

// testAccCheckAPIKeyExists verifies via the API that the key in state exists on
// the server under the expected name.
func testAccCheckAPIKeyExists(t *testing.T, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource %s has no ID", name)
		}

		apiKey, err := queryAPIKey(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if apiKey == nil {
			return fmt.Errorf("API key %s not found on server", rs.Primary.ID)
		}
		if got := apiKey["name"]; got != rs.Primary.Attributes["name"] {
			return fmt.Errorf("server name %v does not match state name %q", got, rs.Primary.Attributes["name"])
		}
		return nil
	}
}

// testAccCheckAPIKeyAttr verifies a single attribute server-side.
func testAccCheckAPIKeyAttr(t *testing.T, name, key string, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		apiKey, err := queryAPIKey(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if apiKey == nil {
			return fmt.Errorf("API key %s not found on server", rs.Primary.ID)
		}
		if got := fmt.Sprint(apiKey[key]); got != fmt.Sprint(want) {
			return fmt.Errorf("server %s = %v, want %v", key, got, want)
		}
		return nil
	}
}

// testAccCheckAPIKeyExpiry verifies the expiry the server holds, which it
// reports as milliseconds since the Unix epoch.
func testAccCheckAPIKeyExpiry(t *testing.T, name string, wantUnixMillis int64) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		apiKey, err := queryAPIKey(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if apiKey == nil {
			return fmt.Errorf("API key %s not found on server", rs.Primary.ID)
		}

		expiresAt, ok := apiKey["expires_at"].(map[string]any)
		if !ok {
			return fmt.Errorf("server expires_at = %v, want a $date wrapper", apiKey["expires_at"])
		}
		if got := fmt.Sprint(expiresAt["$date"]); got != fmt.Sprint(float64(wantUnixMillis)) {
			return fmt.Errorf("server expires_at = %s, want %d", got, wantUnixMillis)
		}
		return nil
	}
}

// testAccCheckAPIKeyDestroy verifies every key in the plan is gone.
func testAccCheckAPIKeyDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if rs.Type != "truenas_api_key" {
				continue
			}

			apiKey, err := queryAPIKey(t, rs.Primary.ID)
			if err != nil {
				return err
			}
			if apiKey != nil {
				return fmt.Errorf("API key %s (%s) still exists after destroy", rs.Primary.ID, name)
			}
		}
		return nil
	}
}

// queryAPIKey returns the key with the given ID, or nil when absent.
func queryAPIKey(t *testing.T, id string) (map[string]any, error) {
	t.Helper()

	raw, err := testAccClient(t).Call(
		context.Background(),
		"api_key.query",
		[]any{[]any{[]any{"id", "=", jsonNumber(id)}}},
	)
	if err != nil {
		return nil, fmt.Errorf("query API key %s: %w", id, err)
	}

	var keys []map[string]any
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("parse API key query response: %w", err)
	}
	if len(keys) == 0 {
		return nil, nil
	}
	return keys[0], nil
}

func TestAccAPIKeyResource(t *testing.T) {
	const resourceName = "truenas_api_key.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAPIKeyDestroy(t),
		Steps: []resource.TestStep{
			// Create a non-expiring key and read it back.
			{
				Config: testAccAPIKeyConfig("tfacc-api-key", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAPIKeyExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "tfacc-api-key"),
					resource.TestCheckResourceAttr(resourceName, "username", testAccAPIUser()),
					resource.TestCheckResourceAttr(resourceName, "store_key", "true"),
					resource.TestCheckResourceAttr(resourceName, "revoked", "false"),
					resource.TestCheckResourceAttr(resourceName, "local", "true"),
					resource.TestCheckNoResourceAttr(resourceName, "expires_at"),
					resource.TestCheckNoResourceAttr(resourceName, "revoked_reason"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "user_identifier"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					// The secret is disclosed once, when the key is created.
					resource.TestMatchResourceAttr(resourceName, "key", regexp.MustCompile(`^\d+-\w+$`)),
					testAccCheckAPIKeyAttr(t, resourceName, "name", "tfacc-api-key"),
					testAccCheckAPIKeyAttr(t, resourceName, "expires_at", "<nil>"),
				),
			},
			// Rename and set an expiry in place, keeping the stored secret.
			{
				Config: testAccAPIKeyConfig("tfacc-api-key-renamed", `
  expires_at = "2035-01-02T15:04:05Z"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAPIKeyExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "name", "tfacc-api-key-renamed"),
					resource.TestCheckResourceAttr(resourceName, "expires_at", "2035-01-02T15:04:05Z"),
					resource.TestMatchResourceAttr(resourceName, "key", regexp.MustCompile(`^\d+-\w+$`)),
					testAccCheckAPIKeyAttr(t, resourceName, "name", "tfacc-api-key-renamed"),
				),
			},
			// The same instant written in a local offset must apply cleanly and
			// keep the configured text, rather than tripping Terraform's
			// consistency check against the UTC form the API reports.
			{
				Config: testAccAPIKeyConfig("tfacc-api-key-renamed", `
  expires_at = "2035-01-02T16:04:05+01:00"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "expires_at", "2035-01-02T16:04:05+01:00"),
					testAccCheckAPIKeyExpiry(t, resourceName, 2051363045000),
				),
			},
			// Clearing the expiry sends an explicit null.
			{
				Config: testAccAPIKeyConfig("tfacc-api-key-renamed", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(resourceName, "expires_at"),
					testAccCheckAPIKeyAttr(t, resourceName, "expires_at", "<nil>"),
				),
			},
			// Import by key ID. The secret cannot be imported, so it is the one
			// attribute the imported state legitimately differs on.
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"},
			},
		},
	})
}

// TestAccAPIKeyResource_StoreKey asserts the lifecycle of an unstored secret:
// it survives the creating apply, leaves state on the next refresh, and can
// only be reinstated by replacing the key.
func TestAccAPIKeyResource_StoreKey(t *testing.T) {
	const resourceName = "truenas_api_key.test"
	var firstID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAPIKeyDestroy(t),
		Steps: []resource.TestStep{
			// The creating apply still yields the secret, so other resources
			// in that apply can consume it.
			{
				Config: testAccAPIKeyConfig("tfacc-api-key-store", `
  store_key = false`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAPIKeyExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "store_key", "false"),
					resource.TestMatchResourceAttr(resourceName, "key", regexp.MustCompile(`^\d+-\w+$`)),
					recordAPIKeyID(resourceName, &firstID),
				),
			},
			// The next refresh drops it, and no plan follows from that.
			{
				Config: testAccAPIKeyConfig("tfacc-api-key-store", `
  store_key = false`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(resourceName, "key"),
					checkAPIKeyIDUnchanged(resourceName, &firstID),
				),
			},
			// Storing the key again is only possible by issuing a new one.
			{
				Config: testAccAPIKeyConfig("tfacc-api-key-store", `
  store_key = true`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAPIKeyExists(t, resourceName),
					resource.TestMatchResourceAttr(resourceName, "key", regexp.MustCompile(`^\d+-\w+$`)),
					checkAPIKeyIDChanged(resourceName, &firstID),
				),
			},
		},
	})
}

// TestAccAPIKeyResource_InvalidExpiry asserts a malformed expiry is rejected
// during plan rather than by the API on apply.
func TestAccAPIKeyResource_InvalidExpiry(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAPIKeyConfig("tfacc-api-key-invalid", `
  expires_at = "2035-01-02"`),
				ExpectError: regexp.MustCompile(`Invalid RFC 3339 Timestamp`),
				PlanOnly:    true,
			},
		},
	})
}

// recordAPIKeyID captures the key ID so a later step can assert on it.
func recordAPIKeyID(name string, into *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		*into = rs.Primary.ID
		return nil
	}
}

// checkAPIKeyIDUnchanged asserts the key was updated in place.
func checkAPIKeyIDUnchanged(name string, previous *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		if rs.Primary.ID != *previous {
			return fmt.Errorf("expected API key %s to be kept, got %s", *previous, rs.Primary.ID)
		}
		return nil
	}
}

// checkAPIKeyIDChanged asserts the key was replaced rather than updated.
func checkAPIKeyIDChanged(name string, previous *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		if rs.Primary.ID == *previous {
			return fmt.Errorf("expected a new API key, but ID %s was reused", rs.Primary.ID)
		}
		return nil
	}
}
