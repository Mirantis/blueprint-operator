package consts

var (

	// The k0s semver regex is the same as the optional v semver regex, but with an optional "+k0s.0" at the end
	K0sSemverRegex = `^[v]?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:\+(k[0-9a-zA-Z-]s+(?:\.[0-9a-zA-Z-]+)*))?$`
)
