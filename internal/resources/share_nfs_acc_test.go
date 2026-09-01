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

// testAccShareNFSDataset renders the dataset every share step exports.
func testAccShareNFSDataset(dataset string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  pool = %[1]q
  path = %[2]q
}
`, testAccPool(), dataset)
}

// testAccShareNFSConfig renders a dataset plus an NFS share exporting it.
// extra is spliced into the share body so steps can vary optional attributes.
func testAccShareNFSConfig(dataset, comment string, readOnly bool, hosts []string, extra string) string {
	quoted := make([]string, len(hosts))
	for i, h := range hosts {
		quoted[i] = fmt.Sprintf("%q", h)
	}

	return testAccProviderConfig() + testAccShareNFSDataset(dataset) + fmt.Sprintf(`
resource "truenas_share_nfs" "test" {
  path    = truenas_dataset.test.full_path
  comment = %[1]q
  ro      = %[2]t

  hosts = [%[3]s]
%[4]s
}
`, comment, readOnly, strings.Join(quoted, ", "), extra)
}

// testAccCheckShareNFSExists verifies via the API that the share in state
// exists on the server and exports the expected path.
func testAccCheckShareNFSExists(t *testing.T, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource %s has no ID", name)
		}

		share, err := queryShareNFS(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if share == nil {
			return fmt.Errorf("NFS share %s not found on server", rs.Primary.ID)
		}
		if got := share["path"]; got != rs.Primary.Attributes["path"] {
			return fmt.Errorf("server path %v does not match state path %q", got, rs.Primary.Attributes["path"])
		}
		return nil
	}
}

// testAccCheckShareNFSAttr verifies a single attribute server-side.
func testAccCheckShareNFSAttr(t *testing.T, name, key string, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		share, err := queryShareNFS(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if share == nil {
			return fmt.Errorf("NFS share %s not found on server", rs.Primary.ID)
		}
		if got := fmt.Sprint(share[key]); got != fmt.Sprint(want) {
			return fmt.Errorf("server %s = %v, want %v", key, got, want)
		}
		return nil
	}
}

// testAccCheckShareNFSDestroy verifies every share in the plan is gone.
func testAccCheckShareNFSDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if rs.Type != "truenas_share_nfs" {
				continue
			}

			share, err := queryShareNFS(t, rs.Primary.ID)
			if err != nil {
				return err
			}
			if share != nil {
				return fmt.Errorf("NFS share %s (%s) still exists after destroy", rs.Primary.ID, name)
			}
		}
		return nil
	}
}

// queryShareNFS returns the share with the given ID, or nil when absent.
func queryShareNFS(t *testing.T, id string) (map[string]any, error) {
	t.Helper()

	raw, err := testAccClient(t).Call(
		context.Background(),
		"sharing.nfs.query",
		[]any{[]any{[]any{"id", "=", jsonNumber(id)}}},
	)
	if err != nil {
		return nil, fmt.Errorf("query NFS share %s: %w", id, err)
	}

	var shares []map[string]any
	if err := json.Unmarshal(raw, &shares); err != nil {
		return nil, fmt.Errorf("parse NFS share query response: %w", err)
	}
	if len(shares) == 0 {
		return nil, nil
	}
	return shares[0], nil
}

// jsonNumber converts a state ID to a number so the API filter matches.
func jsonNumber(id string) json.Number {
	return json.Number(id)
}

func TestAccShareNFSResource(t *testing.T) {
	const resourceName = "truenas_share_nfs.test"
	dataset := "tfacc-share-nfs"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckShareNFSDestroy(t),
		Steps: []resource.TestStep{
			// Create and read back.
			{
				Config: testAccShareNFSConfig(dataset, "acc test share", false, []string{"10.0.0.10"}, `
  security = ["SYS"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckShareNFSExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "comment", "acc test share"),
					resource.TestCheckResourceAttr(resourceName, "ro", "false"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "hosts.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "hosts.0", "10.0.0.10"),
					resource.TestCheckResourceAttr(resourceName, "security.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "security.0", "SYS"),
					resource.TestCheckResourceAttr(resourceName, "networks.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "aliases.#", "0"),
					resource.TestCheckNoResourceAttr(resourceName, "maproot_user"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrPair(resourceName, "path", "truenas_dataset.test", "full_path"),
					testAccCheckShareNFSAttr(t, resourceName, "comment", "acc test share"),
					testAccCheckShareNFSAttr(t, resourceName, "ro", false),
				),
			},
			// Import by share ID.
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update comment, ro, hosts and the root mapping in place.
			{
				Config: testAccShareNFSConfig(dataset, "acc test share updated", true,
					[]string{"10.0.0.10", "10.0.0.11"}, `
  security      = ["SYS"]
  maproot_user  = "root"
  maproot_group = "root"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckShareNFSExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "comment", "acc test share updated"),
					resource.TestCheckResourceAttr(resourceName, "ro", "true"),
					resource.TestCheckResourceAttr(resourceName, "hosts.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "hosts.1", "10.0.0.11"),
					resource.TestCheckResourceAttr(resourceName, "maproot_user", "root"),
					resource.TestCheckResourceAttr(resourceName, "maproot_group", "root"),
					testAccCheckShareNFSAttr(t, resourceName, "comment", "acc test share updated"),
					testAccCheckShareNFSAttr(t, resourceName, "ro", true),
					testAccCheckShareNFSAttr(t, resourceName, "maproot_user", "root"),
				),
			},
			// Clearing the mapping sends an explicit null.
			{
				Config: testAccShareNFSConfig(dataset, "acc test share updated", true,
					[]string{"10.0.0.10", "10.0.0.11"}, `
  security = ["SYS"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(resourceName, "maproot_user"),
					testAccCheckShareNFSAttr(t, resourceName, "maproot_user", "<nil>"),
				),
			},
			// Omitting security and expose_snapshots exercises the schema
			// defaults against the live API.
			{
				Config: testAccShareNFSConfig(dataset, "acc test share defaults", false,
					[]string{"10.0.0.10"}, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckShareNFSExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "security.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "expose_snapshots", "false"),
					testAccCheckShareNFSAttr(t, resourceName, "security", "[]"),
					testAccCheckShareNFSAttr(t, resourceName, "expose_snapshots", false),
				),
			},
		},
	})
}

// TestAccShareNFSResource_MappingValidators asserts the mutually exclusive and
// co-required mapping attributes are rejected during plan, not on apply.
func TestAccShareNFSResource_MappingValidators(t *testing.T) {
	dataset := "tfacc-share-nfs-validators"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccShareNFSConfig(dataset, "invalid", false, nil, `
  maproot_user = "root"
  mapall_user  = "nobody"`),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
				PlanOnly:    true,
			},
			{
				Config: testAccShareNFSConfig(dataset, "invalid", false, nil, `
  maproot_group = "root"`),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
				PlanOnly:    true,
			},
			{
				Config: testAccShareNFSConfig(dataset, "invalid", false, nil, `
  mapall_group = "nogroup"`),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
				PlanOnly:    true,
			},
		},
	})
}
