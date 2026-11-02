module github.com/777genius/plugin-kit-ai

go 1.25.0

toolchain go1.26.10

require (
	github.com/777genius/plugin-kit-ai/install/integrationctl v0.0.0-20260919103549-148794ac5947
	github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins v0.0.0
	github.com/777genius/plugin-kit-ai/sdk v1.3.0
	golang.org/x/sys v0.44.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/creachadair/tomledit v0.0.29 // indirect
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/pelletier/go-toml/v2 v2.3.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/tailscale/hujson v0.0.0-20260302212456-ecc657c15afd // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

replace github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins => ./install/integrationctl/agentplugins

replace github.com/777genius/plugin-kit-ai/install/integrationctl => ./install/integrationctl

replace github.com/777genius/plugin-kit-ai/sdk => ./sdk
