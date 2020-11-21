package webhooks

import (
	"os"
	"strings"

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
	HTTPSURL string `json:"https_url"`
	SSHURL   string `json:"ssh_url"`
	Rev      string `json:"rev"`
	Ref      string `json:"ref"`      // refs/tags/v0.0.1, refs/heads/master
	RefType  string `json:"ref_type"` // tag, branch
	RefName  string `json:"ref_name"`
	Branch   string `json:"branch"`
	Tag      string `json:"tag"`
	Owner    string `json:"repo_owner"`
	Repo     string `json:"repo_name"`
}

// Providers is a map of the git webhook providers
var Providers = make(map[string]func())

// Webhooks is a map of routes
var Webhooks = make(map[string]func(chi.Router))

// Hooks is a channel of incoming webhooks
var Hooks = make(chan Ref)

// Hook will put a Git Ref on the queue
func Hook(r Ref) {
	Hooks <- r
}

// Accept will pop a Git Ref off the queue
func Accept() Ref {
	return <-Hooks
}

// AddProvider registers a git webhook provider
func AddProvider(name string, initProvider func()) {
	Providers[name] = initProvider
}

// AddRouteHandler registers a git webhook route
func AddRouteHandler(name string, route func(router chi.Router)) {
	Webhooks[name] = route
}

// MustRegisterAll registers all webhook route functions
func MustRegisterAll() {
	for _, addHandler := range Providers {
		addHandler()
	}
}

// RouteHandlers registers the webhook functions to the route
func RouteHandlers(r chi.Router) {
	r.Route("/api/webhooks", func(r chi.Router) {
		for provider, handler := range Webhooks {
			r.Route("/"+provider, func(r chi.Router) {
				handler(r)
			})
		}
	})
}

// ParseSecrets grabs secrets from the ENV at runtime
func ParseSecrets(providername, secretList, envname string) [][]byte {
	if 0 == len(secretList) {
		secretList = os.Getenv(envname)
	}
	if 0 == len(secretList) {
		return nil
	}

	var secrets [][]byte
	for _, secret := range strings.Fields(strings.ReplaceAll(secretList, ",", " ")) {
		if len(secret) > 0 {
			secrets = append(secrets, []byte(secret))
		}
	}

	return secrets
}
