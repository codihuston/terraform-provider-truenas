package services

import (
	"errors"
	"strings"

	"github.com/deevus/truenas-go/client"
)

// errnoENOENT is the POSIX ENOENT value the API reports in JSON-RPC error data
// when an instance does not exist.
const errnoENOENT = 2

// isNotFoundError reports whether err is the API's ENOENT signal for a missing
// instance. Only a provable ENOENT counts: generic prose such as the JSON-RPC
// "Method does not exist" reply must surface as a real error rather than being
// mistaken for a deleted object.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var rpcErr *client.JSONRPCError
	if errors.As(err, &rpcErr) {
		if rpcErr.Data == nil {
			return false
		}
		return rpcErr.Data.Error == errnoENOENT || strings.Contains(rpcErr.Data.Reason, "[ENOENT]")
	}

	return strings.Contains(err.Error(), "[ENOENT]")
}

// stringList normalises a nil slice to an empty slice so the API receives `[]`
// rather than `null`, which it rejects.
func stringList(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
