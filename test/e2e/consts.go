package e2e

import "time"

// BoundlessNamespace is the namespace where the boundless operator is installed
const BoundlessNamespace = "boundless-system"

// BoundlessOperatorName is the name of the boundless operator deployment
const BoundlessOperatorName = "boundless-operator-controller-manager"

// DefaultWaitTimeout is the default timeout for waiting for resources to be created/ready/deleted
const DefaultWaitTimeout = time.Minute * 1

// FieldManager is the server-side apply field manager used when applying
// manifests.
const FieldManager = "boundless-e2e-tests"
