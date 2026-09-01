package services

import (
	truenas "github.com/deevus/truenas-go"
	"github.com/deevus/truenas-go/client"
)

// TrueNASServices holds typed service instances for all TrueNAS API namespaces.
// Resources and datasources access services through this registry.
type TrueNASServices struct {
	// Client provides backward-compatible access to the raw client.Client
	// for resources that haven't been migrated to typed services yet.
	// Remove this field once all resources use typed service methods.
	Client client.Client

	APIKey       APIKeyServiceAPI
	App          truenas.AppServiceAPI
	CloudSync    truenas.CloudSyncServiceAPI
	Cron         truenas.CronServiceAPI
	Dataset      truenas.DatasetServiceAPI
	Filesystem   truenas.FilesystemServiceAPI
	Service      SystemServicesAPI
	SharingNFS   SharingNFSServiceAPI
	Snapshot     truenas.SnapshotServiceAPI
	SnapshotTask SnapshotTaskServiceAPI
	Virt         truenas.VirtServiceAPI
	VM           truenas.VMServiceAPI

	// Group and User are implemented in this package because truenas-go does
	// not yet cover the group.* and user.* namespaces.
	Group GroupServiceAPI
	User  UserServiceAPI
}
