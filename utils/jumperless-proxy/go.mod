module github.com/detiber/k8s-jumperless/utils/jumperless-proxy

go 1.24.0

require (
	github.com/creack/pty v1.1.24
	github.com/detiber/k8s-jumperless/utils/jumperless-emulator v0.0.0
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.6
	github.com/spf13/viper v1.20.1
	go.bug.st/serial v1.6.4
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/creack/goselect v0.1.2 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace github.com/detiber/k8s-jumperless/utils/jumperless-emulator => ../jumperless-emulator
