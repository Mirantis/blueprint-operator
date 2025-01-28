package components

import "context"

type Component interface {

	// Name returns the name of the component
	Name() string

	// Images returns the images used by the component
	Images() []string

	// Install installs the component
	Install(ctx context.Context) error

	// Uninstall uninstalls the component
	Uninstall(ctx context.Context) error

	// CheckExists checks if the component exists
	CheckExists(ctx context.Context) (bool, error)
}
