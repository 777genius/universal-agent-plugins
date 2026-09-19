module github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins

go 1.25.0

toolchain go1.25.13

replace github.com/777genius/plugin-kit-ai/install/integrationctl => ../

require (
	github.com/777genius/plugin-kit-ai/install/integrationctl v0.0.0-00010101000000-000000000000
	github.com/dlclark/regexp2 v1.12.0
	github.com/pelletier/go-toml/v2 v2.3.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/tailscale/hujson v0.0.0-20260302212456-ecc657c15afd
	golang.org/x/mod v0.37.0
	golang.org/x/sys v0.42.0
	golang.org/x/text v0.40.0
	gopkg.in/yaml.v3 v3.0.1
)
