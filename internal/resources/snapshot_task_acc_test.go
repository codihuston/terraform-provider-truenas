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

// testAccSnapshotTaskDataset renders the dataset every snapshot-task step
// takes snapshots of.
func testAccSnapshotTaskDataset(dataset string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  pool = %[1]q
  path = %[2]q
}
`, testAccPool(), dataset)
}

// testAccSnapshotTaskConfig renders a dataset plus a periodic snapshot task
// for it. body is spliced into the task so steps can vary its attributes.
func testAccSnapshotTaskConfig(dataset, body string) string {
	return testAccProviderConfig() + testAccSnapshotTaskDataset(dataset) + fmt.Sprintf(`
resource "truenas_snapshot_task" "test" {
  dataset = truenas_dataset.test.id
%[1]s
}
`, body)
}

// testAccCheckSnapshotTaskExists verifies via the API that the task in state
// exists on the server and targets the expected dataset.
func testAccCheckSnapshotTaskExists(t *testing.T, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource %s has no ID", name)
		}

		task, err := querySnapshotTask(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("periodic snapshot task %s not found on server", rs.Primary.ID)
		}
		if got := task["dataset"]; got != rs.Primary.Attributes["dataset"] {
			return fmt.Errorf("server dataset %v does not match state dataset %q", got, rs.Primary.Attributes["dataset"])
		}
		return nil
	}
}

// testAccCheckSnapshotTaskAttr verifies a single attribute server-side.
func testAccCheckSnapshotTaskAttr(t *testing.T, name, key string, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		task, err := querySnapshotTask(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("periodic snapshot task %s not found on server", rs.Primary.ID)
		}
		if got := fmt.Sprint(task[key]); got != fmt.Sprint(want) {
			return fmt.Errorf("server %s = %v, want %v", key, got, want)
		}
		return nil
	}
}

// testAccCheckSnapshotTaskScheduleAttr verifies a single schedule field
// server-side.
func testAccCheckSnapshotTaskScheduleAttr(t *testing.T, name, key, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}

		task, err := querySnapshotTask(t, rs.Primary.ID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("periodic snapshot task %s not found on server", rs.Primary.ID)
		}

		schedule, ok := task["schedule"].(map[string]any)
		if !ok {
			return fmt.Errorf("server schedule is %T, want an object", task["schedule"])
		}
		if got := fmt.Sprint(schedule[key]); got != want {
			return fmt.Errorf("server schedule.%s = %v, want %v", key, got, want)
		}
		return nil
	}
}

// testAccCheckSnapshotTaskDestroy verifies every task in the plan is gone.
func testAccCheckSnapshotTaskDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for name, rs := range s.RootModule().Resources {
			if rs.Type != "truenas_snapshot_task" {
				continue
			}

			task, err := querySnapshotTask(t, rs.Primary.ID)
			if err != nil {
				return err
			}
			if task != nil {
				return fmt.Errorf("periodic snapshot task %s (%s) still exists after destroy", rs.Primary.ID, name)
			}
		}
		return nil
	}
}

// querySnapshotTask returns the task with the given ID, or nil when absent.
func querySnapshotTask(t *testing.T, id string) (map[string]any, error) {
	t.Helper()

	raw, err := testAccClient(t).Call(
		context.Background(),
		"pool.snapshottask.query",
		[]any{[]any{[]any{"id", "=", jsonNumber(id)}}},
	)
	if err != nil {
		return nil, fmt.Errorf("query periodic snapshot task %s: %w", id, err)
	}

	var tasks []map[string]any
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return nil, fmt.Errorf("parse periodic snapshot task query response: %w", err)
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return tasks[0], nil
}

func TestAccSnapshotTaskResource(t *testing.T) {
	const resourceName = "truenas_snapshot_task.test"
	dataset := "tfacc-snapshot-task"
	excluded := fmt.Sprintf("%s/%s/scratch", testAccPool(), dataset)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSnapshotTaskDestroy(t),
		Steps: []resource.TestStep{
			// Create and read back.
			{
				Config: testAccSnapshotTaskConfig(dataset, `
  recursive      = true
  naming_schema  = "tfacc-%Y-%m-%d_%H-%M"
  lifetime_value = 4
  lifetime_unit  = "WEEK"

  schedule = {
    minute = "0"
    hour   = "2"
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSnapshotTaskExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "recursive", "true"),
					resource.TestCheckResourceAttr(resourceName, "naming_schema", "tfacc-%Y-%m-%d_%H-%M"),
					resource.TestCheckResourceAttr(resourceName, "lifetime_value", "4"),
					resource.TestCheckResourceAttr(resourceName, "lifetime_unit", "WEEK"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "allow_empty", "true"),
					resource.TestCheckResourceAttr(resourceName, "exclude.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "vmware_sync", "false"),
					resource.TestCheckResourceAttr(resourceName, "schedule.minute", "0"),
					resource.TestCheckResourceAttr(resourceName, "schedule.hour", "2"),
					// Unset schedule fields fall back to the API's defaults.
					resource.TestCheckResourceAttr(resourceName, "schedule.dom", "*"),
					resource.TestCheckResourceAttr(resourceName, "schedule.month", "*"),
					resource.TestCheckResourceAttr(resourceName, "schedule.dow", "*"),
					resource.TestCheckResourceAttr(resourceName, "schedule.begin", "00:00"),
					resource.TestCheckResourceAttr(resourceName, "schedule.end", "23:59"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrPair(resourceName, "dataset", "truenas_dataset.test", "id"),
					testAccCheckSnapshotTaskAttr(t, resourceName, "recursive", true),
					testAccCheckSnapshotTaskAttr(t, resourceName, "lifetime_value", 4),
					testAccCheckSnapshotTaskScheduleAttr(t, resourceName, "hour", "2"),
				),
			},
			// Import by task ID.
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update the retention, schedule and enabled flag in place, and
			// add an exclusion the recursive task honours.
			{
				Config: testAccSnapshotTaskConfig(dataset, `
  recursive      = true
  exclude        = ["`+excluded+`"]
  naming_schema  = "tfacc-%Y-%m-%d_%H-%M"
  lifetime_value = 10
  lifetime_unit  = "DAY"
  allow_empty    = false
  enabled        = false

  schedule = {
    minute = "30"
    hour   = "4"
    dow    = "7"
    begin  = "01:00"
    end    = "22:00"
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSnapshotTaskExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "lifetime_value", "10"),
					resource.TestCheckResourceAttr(resourceName, "lifetime_unit", "DAY"),
					resource.TestCheckResourceAttr(resourceName, "allow_empty", "false"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "false"),
					resource.TestCheckResourceAttr(resourceName, "exclude.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "schedule.minute", "30"),
					resource.TestCheckResourceAttr(resourceName, "schedule.dow", "7"),
					resource.TestCheckResourceAttr(resourceName, "schedule.begin", "01:00"),
					resource.TestCheckResourceAttr(resourceName, "schedule.end", "22:00"),
					testAccCheckSnapshotTaskAttr(t, resourceName, "lifetime_value", 10),
					testAccCheckSnapshotTaskAttr(t, resourceName, "lifetime_unit", "DAY"),
					testAccCheckSnapshotTaskAttr(t, resourceName, "enabled", false),
					testAccCheckSnapshotTaskScheduleAttr(t, resourceName, "begin", "01:00"),
				),
			},
			// Omitting every optional attribute exercises the schema defaults
			// against the live API.
			{
				Config: testAccSnapshotTaskConfig(dataset, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSnapshotTaskExists(t, resourceName),
					resource.TestCheckResourceAttr(resourceName, "recursive", "false"),
					resource.TestCheckResourceAttr(resourceName, "exclude.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "lifetime_value", "2"),
					resource.TestCheckResourceAttr(resourceName, "lifetime_unit", "WEEK"),
					resource.TestCheckResourceAttr(resourceName, "naming_schema", "auto-%Y-%m-%d_%H-%M"),
					resource.TestCheckResourceAttr(resourceName, "allow_empty", "true"),
					resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "schedule.minute", "00"),
					resource.TestCheckResourceAttr(resourceName, "schedule.hour", "*"),
					testAccCheckSnapshotTaskAttr(t, resourceName, "naming_schema", "auto-%Y-%m-%d_%H-%M"),
					testAccCheckSnapshotTaskScheduleAttr(t, resourceName, "minute", "00"),
				),
			},
		},
	})
}

// TestAccSnapshotTaskResource_LifetimeUnitValidator asserts an invalid
// retention unit is rejected during plan, not on apply.
func TestAccSnapshotTaskResource_LifetimeUnitValidator(t *testing.T) {
	dataset := "tfacc-snapshot-task-validators"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSnapshotTaskConfig(dataset, `  lifetime_unit = "FORTNIGHT"`),
				ExpectError: regexp.MustCompile(`Invalid Attribute Value Match`),
				PlanOnly:    true,
			},
		},
	})
}
