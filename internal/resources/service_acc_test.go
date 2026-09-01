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

// testAccServiceConfig renders a service resource for the given name.
func testAccServiceConfig(name string, enable, running bool) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "truenas_service" "test" {
  name    = %[1]q
  enable  = %[2]t
  running = %[3]t
}
`, name, enable, running)
}

// testAccCheckServiceState verifies the service's boot and run state
// server-side, independent of Terraform state.
func testAccCheckServiceState(t *testing.T, name string, enable, running bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		svc, err := queryService(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if svc == nil {
			return fmt.Errorf("service %s not found on server", rs.Primary.ID)
		}

		if got := svc["enable"]; got != enable {
			return fmt.Errorf("server enable = %v, want %v", got, enable)
		}
		if got := svc["state"] == "RUNNING"; got != running {
			return fmt.Errorf("server state = %v, want running %v", svc["state"], running)
		}
		return nil
	}
}

// testAccCheckServiceDestroy verifies every managed service was stopped and
// disabled, which is what destroying this resource means.
func testAccCheckServiceDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if rs.Type != "truenas_service" {
				continue
			}

			svc, err := queryService(t, rs.Primary.ID)
			if err != nil {
				return err
			}
			if svc == nil {
				return fmt.Errorf("service %s (%s) no longer exists on the server", rs.Primary.ID, name)
			}
			if svc["enable"] != false {
				return fmt.Errorf("service %s still enabled after destroy", rs.Primary.ID)
			}
			if svc["state"] == "RUNNING" {
				return fmt.Errorf("service %s still running after destroy", rs.Primary.ID)
			}
		}
		return nil
	}
}

// queryService returns the service with the given name, or nil when absent.
func queryService(t *testing.T, name string) (map[string]any, error) {
	t.Helper()

	raw, err := testAccClient(t).Call(
		context.Background(),
		"service.query",
		[]any{[]any{[]any{"service", "=", name}}},
	)
	if err != nil {
		return nil, fmt.Errorf("query service %s: %w", name, err)
	}

	var svcs []map[string]any
	if err := json.Unmarshal(raw, &svcs); err != nil {
		return nil, fmt.Errorf("parse service query response: %w", err)
	}
	if len(svcs) == 0 {
		return nil, nil
	}
	return svcs[0], nil
}

// TestAccServiceResource drives the NFS service through every combination of
// boot and run state, then checks that destroy leaves it stopped and disabled.
func TestAccServiceResource(t *testing.T) {
	const resourceName = "truenas_service.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceDestroy(t),
		Steps: []resource.TestStep{
			// Adopt the service and bring it up.
			{
				Config: testAccServiceConfig("nfs", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", "nfs"),
					resource.TestCheckResourceAttr(resourceName, "name", "nfs"),
					resource.TestCheckResourceAttr(resourceName, "enable", "true"),
					resource.TestCheckResourceAttr(resourceName, "running", "true"),
					testAccCheckServiceState(t, resourceName, true, true),
				),
			},
			// Import by service name.
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateId:     "nfs",
				ImportStateVerify: true,
			},
			// Stop the service but leave it enabled at boot.
			{
				Config: testAccServiceConfig("nfs", true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "enable", "true"),
					resource.TestCheckResourceAttr(resourceName, "running", "false"),
					testAccCheckServiceState(t, resourceName, true, false),
				),
			},
			// Run the service without enabling it at boot.
			{
				Config: testAccServiceConfig("nfs", false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "enable", "false"),
					resource.TestCheckResourceAttr(resourceName, "running", "true"),
					testAccCheckServiceState(t, resourceName, false, true),
				),
			},
			// Omitting both attributes falls back to the schema defaults.
			{
				Config: testAccProviderConfig() + `
resource "truenas_service" "test" {
  name = "nfs"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "enable", "true"),
					resource.TestCheckResourceAttr(resourceName, "running", "true"),
					testAccCheckServiceState(t, resourceName, true, true),
				),
			},
		},
	})
}

// TestAccServiceResource_UnknownName asserts that a service the appliance does
// not offer fails with the list of names it does.
func TestAccServiceResource_UnknownName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceConfig("definitely-not-a-service", true, true),
				// Terraform hard-wraps diagnostics, so the list may be split
				// across lines.
				ExpectError: regexp.MustCompile(`Known\s+services:[\s\S]*nfs`),
			},
		},
	})
}
