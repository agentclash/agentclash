module github.com/agentclash/agentclash/cli

go 1.25.5

require (
	github.com/AlecAivazis/survey/v2 v2.3.7
	github.com/bmatcuk/doublestar/v4 v4.10.0
	github.com/briandowns/spinner v1.23.2
	github.com/fatih/color v1.19.0
	github.com/google/jsonschema-go v0.3.0
	github.com/itchyny/gojq v0.12.19
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	golang.org/x/term v0.42.0
	golang.org/x/text v0.37.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	modernc.org/libc v1.66.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.38.2 // indirect
)

require (
	github.com/agentclash/agentclash/runtime v0.0.0
	github.com/google/uuid v1.6.0
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/itchyny/timefmt-go v0.1.8 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.21 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	golang.org/x/sys v0.45.0 // indirect
)

replace github.com/agentclash/agentclash/runtime => ../runtime
