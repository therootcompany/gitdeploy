package webhooks

import (
	"github.com/go-chi/chi"
)

// Ref represents typical git webhook info such as:
//     HTTPSURL ex: https://git@git.example.com/example/example.git
//     SSHURL   ex: ssh://git@git.example.com/example/example.git
//     Rev      ex: 00000000
//     Ref      ex: /refs/heads/master
//     Branch   ex: master
//     Repo     ex: example
//     Org      ex: example
type Ref struct {
	HTTPSURL string
	SSHURL   string
	Rev      string
	Ref      string
	RefType  string // tags, heads
	RefName  string
	Branch   string
	Tag      string
	Owner    string
	Repo     string
}

var Providers = make(map[string]func())
var Webhooks = make(map[string]func(chi.Router))

var hooks = make(chan Ref)

func Hook(r Ref) {
	hooks <- r
}

func Accept() Ref {
	return <-hooks
}

func AddProvider(name string, initProvider func()) {
	Providers[name] = initProvider
}

func AddRouteHandler(name string, route func(router chi.Router)) {
	Webhooks[name] = route
}

func MustRegisterAll() {
	for _, addHandler := range Providers {
		addHandler()
	}
}

func RouteHandlers(r chi.Router) {
	r.Route("/api/webhooks", func(r chi.Router) {
		for provider, handler := range Webhooks {
			r.Route("/"+provider, func(r chi.Router) {
				handler(r)
			})
		}
	})
}
