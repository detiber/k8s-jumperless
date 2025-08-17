module github.com/detiber/k8s-jumperless/utils/jumperless-proxy

go 1.24.0

require (
	github.com/creack/pty v1.1.24
	github.com/detiber/k8s-jumperless/utils/jumperless-emulator v0.0.0
	go.bug.st/serial v1.6.4
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/creack/goselect v0.1.2 // indirect
	golang.org/x/sys v0.19.0 // indirect
)

replace github.com/detiber/k8s-jumperless/utils/jumperless-emulator => ../jumperless-emulator
