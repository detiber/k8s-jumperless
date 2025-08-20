module github.com/detiber/k8s-jumperless/utils/proxy

go 1.25.0

require (
	github.com/creack/pty v1.1.24
	github.com/detiber/k8s-jumperless v0.0.0-00010101000000-000000000000
	github.com/detiber/k8s-jumperless/utils/emulator v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.6
	github.com/spf13/viper v1.20.1
	go.bug.st/serial v1.6.4
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/charmbracelet/x/ansi v0.10.1 // indirect
	github.com/creack/goselect v0.1.2 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	k8s.io/apimachinery v0.33.4 // indirect
)

replace github.com/detiber/k8s-jumperless/utils/emulator => ../emulator

replace github.com/detiber/k8s-jumperless => ../../
