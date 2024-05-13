package e2e

import "time"

// DefaultWaitTimeout is the default timeout for waiting for resources to be created/ready/deleted
const DefaultWaitTimeout = time.Minute * 2

// FieldManager is the server-side apply field manager used when applying manifests.
const FieldManager = "boundless-e2e-tests"
