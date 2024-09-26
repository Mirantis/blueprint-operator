package v1alpha1

import (
	"fmt"
	"runtime"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var (
	// Hack to get the path to this file to use as an "existing" config file
	_, thisFile, _, _ = runtime.Caller(0)
)

// TestHostsValidateRole tests the validation of a Host's role
func TestHostsValidateRole(t *testing.T) {
	tests := map[string]struct {
		role string
		want types.GomegaMatcher
	}{
		"valid role": {role: nodeRoles[0], want: BeNil()},
		"wrong role": {role: "janitor", want: Equal(fmt.Errorf("invalid hosts.role: janitor\nValid hosts.role values: [single controller worker controller+worker]"))},
		"no role":    {role: "", want: Equal(fmt.Errorf("hosts.role field cannot be left blank"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			host := Host{
				Role: tc.role,
			}
			actual := host.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestSSHHostValidateAddress tests the validation of a SSHHost's address
func TestSSHHostValidateAddress(t *testing.T) {
	tests := map[string]struct {
		address string
		want    types.GomegaMatcher
	}{
		"valid IP address":       {address: "192.168.1.1", want: BeNil()},
		"valid hostname address": {address: "bobs.machine.7", want: BeNil()},
		"no address":             {address: "", want: Equal(fmt.Errorf("hosts.ssh.address field cannot be left empty"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			sshHost := SSHHost{
				Address: tc.address,
				KeyPath: thisFile, // This is required for Validate() to work but not tested here
				Port:    22,       // This is required for Validate() to work but not tested here
				User:    "root",   // This is required for Validate() to work but not tested here
			}
			actual := sshHost.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestSSHHostValidateKeypath tests the validation of a SSHHost's keypath
func TestSSHHostValidateKeypath(t *testing.T) {
	tests := map[string]struct {
		keypath string
		want    types.GomegaMatcher
	}{
		"valid filepath":   {keypath: thisFile, want: BeNil()},
		"invalid filepath": {keypath: "/tmp/keypath", want: Equal(fmt.Errorf("hosts.ssh.keypath does not exist: /tmp/keypath"))},
		"no keypath":       {keypath: "", want: Equal(fmt.Errorf("hosts.ssh.keypath field cannot be left empty"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			sshHost := SSHHost{
				Address: "localhost", // This is required for Validate() to work but not tested here
				KeyPath: tc.keypath,
				Port:    22,     // This is required for Validate() to work but not tested here
				User:    "root", // This is required for Validate() to work but not tested here
			}
			actual := sshHost.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestSSHHostValidatePort tests the validation of a SSHHost's port
func TestSSHHostValidatePort(t *testing.T) {
	tests := map[string]struct {
		port int
		want types.GomegaMatcher
	}{
		"valid port":       {port: 22, want: BeNil()},
		"above port range": {port: 65536, want: Equal(fmt.Errorf("hosts.ssh.port outside of valid range 0-65535"))},
		"below port range": {port: -1, want: Equal(fmt.Errorf("hosts.ssh.port outside of valid range 0-65535"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			sshHost := SSHHost{
				Address: "localhost", // This is required for Validate() to work but not tested here
				KeyPath: thisFile,    // This is required for Validate() to work but not tested here
				Port:    tc.port,
				User:    "root", // This is required for Validate() to work but not tested here
			}
			actual := sshHost.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}

// TestSSHHostValidateUser tests the validation of a SSHHost's user
func TestSSHHostValidateUser(t *testing.T) {
	tests := map[string]struct {
		user string
		want types.GomegaMatcher
	}{
		"valid user": {user: "Bob", want: BeNil()},
		"no user":    {user: "", want: Equal(fmt.Errorf("hosts.ssh.user cannot be left empty"))},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set up the test environment
			g := NewWithT(t)

			// Run the method under test
			sshHost := SSHHost{
				Address: "localhost", // This is required for Validate() to work but not tested here
				KeyPath: thisFile,    // This is required for Validate() to work but not tested here
				Port:    22,          // This is required for Validate() to work but not tested here
				User:    tc.user,
			}
			actual := sshHost.Validate()

			// Check the results
			g.Expect(actual).Should(tc.want)

		})
	}
}
