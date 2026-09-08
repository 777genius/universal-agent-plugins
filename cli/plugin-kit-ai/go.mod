module github.com/777genius/plugin-kit-ai/cli

go 1.25.8

toolchain go1.25.13

replace github.com/777genius/plugin-kit-ai/sdk => ../../sdk

replace github.com/777genius/plugin-kit-ai/install/integrationctl => ../../install/integrationctl

replace github.com/777genius/plugin-kit-ai/plugininstall => ../../install/plugininstall

require (
	charm.land/bubbles/v2 v2.0.0
	charm.land/bubbletea/v2 v2.0.2
	charm.land/huh/v2 v2.0.3
	github.com/777genius/plugin-kit-ai/install/integrationctl v0.0.0
	github.com/777genius/plugin-kit-ai/plugininstall v0.0.0
	github.com/777genius/plugin-kit-ai/sdk v0.0.0
	github.com/charmbracelet/colorprofile v0.4.2
	github.com/muesli/cancelreader v0.2.2
	github.com/pelletier/go-toml/v2 v2.3.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.9
	github.com/tailscale/hujson v0.0.0-20260302212456-ecc657c15afd
	golang.org/x/sys v0.42.0
	golang.org/x/term v0.34.0
	golang.org/x/text v0.40.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	charm.land/lipgloss/v2 v2.0.1 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/catppuccin/go v0.2.0 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260205113103-524a6607adb8 // indirect
	github.com/charmbracelet/x/ansi v0.11.6 // indirect
	github.com/charmbracelet/x/exp/ordered v0.1.0 // indirect
	github.com/charmbracelet/x/exp/strings v0.0.0-20240722160745-212f7b056ed0 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/dlclark/regexp2 v1.12.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.20 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.37.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
)
