package resources_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// envSSHCredential names an existing SSH_CREDENTIALS keychain credential to
// replicate to. When it is unset the test provisions a loopback credential
// against the acceptance target itself and removes it afterwards.
const envSSHCredential = "TRUENAS_ACC_SSH_CREDENTIAL"

// replicationSSHUser is the account the provisioned loopback credential
// authenticates as. Replication invokes `zfs` on the target, which needs root
// unless the task is configured for passwordless sudo.
const replicationSSHUser = "root"

// testAccReplicationTaskDatasets renders the source and target datasets every
// step replicates between.
func testAccReplicationTaskDatasets(source, target string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "source" {
  pool = %[1]q
  path = %[2]q
}

resource "truenas_dataset" "target" {
  pool = %[1]q
  path = %[3]q
}
`, testAccPool(), source, target)
}

// testAccReplicationTaskConfig renders a nightly push between the two datasets.
// extra is spliced into the task body so steps can vary optional attributes.
func testAccReplicationTaskConfig(source, target, name string, credential int64, extra string) string {
	return testAccProviderConfig() + testAccReplicationTaskDatasets(source, target) + fmt.Sprintf(`
resource "truenas_replication_task" "test" {
  name            = %[1]q
  ssh_credentials = %[2]d

  source_datasets = ["${truenas_dataset.source.pool}/${truenas_dataset.source.path}"]
  target_dataset  = "${truenas_dataset.target.pool}/${truenas_dataset.target.path}"

  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]

  retention_policy = "CUSTOM"
  lifetime_value   = 2
  lifetime_unit    = "WEEK"

  schedule {
    minute = "0"
    hour   = "3"
  }
%[3]s
}
`, name, credential, extra)
}

func TestAccReplicationTaskResource_basic(t *testing.T) {
	source := "tf-acc-repl-src"
	target := "tf-acc-repl-dst"
	credential := testAccSSHCredential(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationTaskDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccReplicationTaskConfig(source, target, "tf-acc-nightly", credential, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckReplicationTaskExists(t, "truenas_replication_task.test"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "name", "tf-acc-nightly"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "direction", "PUSH"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "transport", "SSH"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "retention_policy", "CUSTOM"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "lifetime_value", 2),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "auto", true),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "enabled", true),
					testAccCheckReplicationTaskSchedule(t, "truenas_replication_task.test", "0", "3"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "auto", "true"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "readonly", "SET"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "retries", "5"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "schedule.dom", "*"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "schedule.begin", "00:00"),
					resource.TestCheckResourceAttrSet("truenas_replication_task.test", "state"),
				),
			},
			{
				// Every mutable dimension at once: identity, schedule, retention,
				// SSH tuning and the enabled flag.
				Config: testAccReplicationTaskConfig(source, target, "tf-acc-nightly-renamed", credential, `
  enabled          = false
  recursive        = true
  compression      = "LZ4"
  speed_limit      = 1048576
  logging_level    = "INFO"
  readonly         = "IGNORE"
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckReplicationTaskExists(t, "truenas_replication_task.test"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "name", "tf-acc-nightly-renamed"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "enabled", false),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "recursive", true),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "compression", "LZ4"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "speed_limit", 1048576),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "logging_level", "INFO"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.test", "readonly", "IGNORE"),
				),
			},
			{
				ResourceName:      "truenas_replication_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccReplicationTaskResource_manual covers a task with no schedule: the
// provider must report auto = false and the server must hold no cron.
func TestAccReplicationTaskResource_manual(t *testing.T) {
	source := "tf-acc-repl-manual-src"
	target := "tf-acc-repl-manual-dst"
	credential := testAccSSHCredential(t)

	config := testAccProviderConfig() + testAccReplicationTaskDatasets(source, target) + fmt.Sprintf(`
resource "truenas_replication_task" "manual" {
  name            = "tf-acc-manual"
  ssh_credentials = %[1]d

  source_datasets = ["${truenas_dataset.source.pool}/${truenas_dataset.source.path}"]
  target_dataset  = "${truenas_dataset.target.pool}/${truenas_dataset.target.path}"

  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]

  retention_policy = "SOURCE"
}
`, credential)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationTaskDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckReplicationTaskExists(t, "truenas_replication_task.manual"),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.manual", "auto", false),
					testAccCheckReplicationTaskAttr(t, "truenas_replication_task.manual", "schedule", nil),
					resource.TestCheckNoResourceAttr("truenas_replication_task.manual", "schedule.hour"),
					resource.TestCheckResourceAttr("truenas_replication_task.manual", "auto", "false"),
				),
			},
		},
	})
}

// TestAccReplicationTaskResource_validation asserts the cross-field rules fail
// during plan, before anything reaches the server.
func TestAccReplicationTaskResource_validation(t *testing.T) {
	base := testAccProviderConfig() + `
resource "truenas_replication_task" "invalid" {
  name = "tf-acc-invalid"

  source_datasets = ["tank/nope"]
  target_dataset  = "tank/nope-backup"

  also_include_naming_schema = ["auto-%Y-%m-%d_%H-%M"]
`

	tests := []struct {
		name   string
		body   string
		expect *regexp.Regexp
	}{
		{
			name:   "missing ssh credentials",
			body:   "  retention_policy = \"NONE\"\n}\n",
			expect: regexp.MustCompile(`Missing SSH Credentials`),
		},
		{
			name:   "custom retention without lifetime",
			body:   "  ssh_credentials = 1\n  retention_policy = \"CUSTOM\"\n}\n",
			expect: regexp.MustCompile(`Missing Snapshot Lifetime`),
		},
		{
			name: "lifetime without custom retention",
			body: "  ssh_credentials = 1\n  retention_policy = \"NONE\"\n" +
				"  lifetime_value = 2\n  lifetime_unit = \"WEEK\"\n}\n",
			expect: regexp.MustCompile(`Unexpected Snapshot Lifetime`),
		},
		{
			name: "exclude without recursive",
			body: "  ssh_credentials = 1\n  retention_policy = \"NONE\"\n" +
				"  exclude = [\"tank/nope/scratch\"]\n}\n",
			expect: regexp.MustCompile(`Exclusions Require Recursive Replication`),
		},
		{
			name:   "unsupported direction",
			body:   "  ssh_credentials = 1\n  retention_policy = \"NONE\"\n  direction = \"PULL\"\n}\n",
			expect: regexp.MustCompile(`Attribute direction value must be one of`),
		},
		{
			name:   "unsupported transport",
			body:   "  ssh_credentials = 1\n  retention_policy = \"NONE\"\n  transport = \"LOCAL\"\n}\n",
			expect: regexp.MustCompile(`Attribute transport value must be one of`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      base + tt.body,
					PlanOnly:    true,
					ExpectError: tt.expect,
				}},
			})
		})
	}
}

// testAccCheckReplicationTaskExists verifies via the API that the task in state
// exists on the server and pushes from the expected source.
func testAccCheckReplicationTaskExists(t *testing.T, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource %s has no ID", name)
		}

		task, err := queryReplicationTask(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("replication task %s not found on server", rs.Primary.ID)
		}

		sources, _ := task["source_datasets"].([]any)
		if len(sources) != 1 || fmt.Sprint(sources[0]) != rs.Primary.Attributes["source_datasets.0"] {
			return fmt.Errorf(
				"server source_datasets %v do not match state %q",
				sources, rs.Primary.Attributes["source_datasets.0"],
			)
		}
		return nil
	}
}

// testAccCheckReplicationTaskAttr verifies a single attribute server-side.
// A want of nil asserts the server reports the field as null.
func testAccCheckReplicationTaskAttr(t *testing.T, name, key string, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		task, err := queryReplicationTask(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("replication task %s not found on server", rs.Primary.ID)
		}

		got := task[key]
		if want == nil {
			if got != nil {
				return fmt.Errorf("server %s = %v, want null", key, got)
			}
			return nil
		}
		if replicationAttrString(got) != replicationAttrString(want) {
			return fmt.Errorf("server %s = %v, want %v", key, got, want)
		}
		return nil
	}
}

// replicationAttrString renders an attribute for comparison. JSON decodes every
// number to float64, so integral values are rendered without an exponent to
// match the plain integers the test asserts.
func replicationAttrString(v any) string {
	if f, ok := v.(float64); ok && f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return fmt.Sprint(v)
}

// testAccCheckReplicationTaskSchedule verifies the cron the server stored.
func testAccCheckReplicationTaskSchedule(t *testing.T, name, minute, hour string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		task, err := queryReplicationTask(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("replication task %s not found on server", rs.Primary.ID)
		}

		schedule, ok := task["schedule"].(map[string]any)
		if !ok {
			return fmt.Errorf("server schedule %v is not an object", task["schedule"])
		}
		if fmt.Sprint(schedule["minute"]) != minute || fmt.Sprint(schedule["hour"]) != hour {
			return fmt.Errorf("server schedule %v, want minute %s hour %s", schedule, minute, hour)
		}
		return nil
	}
}

// testAccCheckReplicationTaskDestroy verifies every task in the plan is gone.
func testAccCheckReplicationTaskDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if rs.Type != "truenas_replication_task" {
				continue
			}

			task, err := queryReplicationTask(t, rs.Primary.ID)
			if err != nil {
				return err
			}
			if task != nil {
				return fmt.Errorf("replication task %s (%s) still exists after destroy", rs.Primary.ID, name)
			}
		}
		return nil
	}
}

// queryReplicationTask returns the task with the given ID, or nil when absent.
func queryReplicationTask(t *testing.T, id string) (map[string]any, error) {
	t.Helper()

	raw, err := testAccClient(t).Call(
		context.Background(),
		"replication.query",
		[]any{[]any{[]any{"id", "=", jsonNumber(id)}}},
	)
	if err != nil {
		return nil, fmt.Errorf("query replication task %s: %w", id, err)
	}

	var tasks []map[string]any
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return nil, fmt.Errorf("parse replication task query: %w", err)
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return tasks[0], nil
}

// testAccSSHCredential returns the keychain credential ID replication tasks
// target. It honours TRUENAS_ACC_SSH_CREDENTIAL when set; otherwise it
// provisions a loopback credential — a generated key pair authorised for root,
// pointed back at the acceptance target — and registers cleanup. Replication
// runs `zfs` on the far side, so the credential uses root rather than the API
// user, matching how TrueNAS documents replication credentials.
//
// Deleting the key pair cascades to the SSH credential built on top of it, so
// one delete tears the whole fixture down.
func testAccSSHCredential(t *testing.T) int64 {
	t.Helper()

	if v := os.Getenv(envSSHCredential); v != "" {
		var id int64
		if _, err := fmt.Sscanf(v, "%d", &id); err != nil {
			t.Fatalf("%s must be a numeric keychain credential ID: %s", envSSHCredential, err)
		}
		return id
	}

	if os.Getenv("TF_ACC") == "" {
		return 0
	}

	ctx := context.Background()
	client := testAccClient(t)

	var keyPair map[string]any
	testAccCall(t, ctx, &keyPair, "keychaincredential.generate_ssh_key_pair", nil)

	var pair map[string]any
	testAccCall(t, ctx, &pair, "keychaincredential.create", map[string]any{
		"name":       "tf-acc-replication-key",
		"type":       "SSH_KEY_PAIR",
		"attributes": keyPair,
	})
	pairID := jsonFieldInt64(t, pair, "id")

	t.Cleanup(func() {
		_, _ = client.Call(ctx, "keychaincredential.delete", []any{pairID, map[string]any{"cascade": true}})
	})

	// The replication task authenticates as root over the loopback connection,
	// so that account has to trust the generated key. It is appended to any key
	// root already had — which may be the one this test suite itself connects
	// with — and the original value is restored afterwards.
	var users []map[string]any
	testAccCall(t, ctx, &users, "user.query", []any{[]any{[]any{"username", "=", replicationSSHUser}}})
	if len(users) == 0 {
		t.Fatalf("user %q not found on the acceptance target", replicationSSHUser)
	}
	userID := jsonFieldInt64(t, users[0], "id")
	previousKey, _ := users[0]["sshpubkey"].(string)

	authorized := strings.TrimSpace(fmt.Sprint(keyPair["public_key"]))
	if trimmed := strings.TrimSpace(previousKey); trimmed != "" {
		authorized = trimmed + "\n" + authorized
	}

	var updated map[string]any
	testAccCall(t, ctx, &updated, "user.update", []any{userID, map[string]any{"sshpubkey": authorized}})

	t.Cleanup(func() {
		_, _ = client.Call(ctx, "user.update", []any{userID, map[string]any{"sshpubkey": users[0]["sshpubkey"]}})
	})

	var hostKey string
	testAccCall(t, ctx, &hostKey, "keychaincredential.remote_ssh_host_key_scan", map[string]any{
		"host": "localhost",
		"port": 22,
	})

	var credential map[string]any
	testAccCall(t, ctx, &credential, "keychaincredential.create", map[string]any{
		"name": "tf-acc-replication-target",
		"type": "SSH_CREDENTIALS",
		"attributes": map[string]any{
			"host":            "localhost",
			"port":            22,
			"username":        replicationSSHUser,
			"private_key":     pairID,
			"remote_host_key": strings.TrimSpace(hostKey),
		},
	})

	return jsonFieldInt64(t, credential, "id")
}

// testAccCall makes an API call against the acceptance target and decodes the
// result into out, failing the test on any error.
func testAccCall(t *testing.T, ctx context.Context, out any, method string, params any) {
	t.Helper()

	raw, err := testAccClient(t).Call(ctx, method, params)
	if err != nil {
		t.Fatalf("%s: %s", method, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("parse %s response: %s", method, err)
	}
}

// jsonFieldInt64 reads a numeric field out of a decoded API object.
func jsonFieldInt64(t *testing.T, obj map[string]any, field string) int64 {
	t.Helper()

	value, ok := obj[field].(float64)
	if !ok {
		t.Fatalf("expected numeric %q, got %v", field, obj[field])
	}
	return int64(value)
}
