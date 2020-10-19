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
	HTTPSURL string `json:"clone_url"`
	SSHURL   string `json:"-"`
	Rev      string `json:"-"`
	Ref      string `json:"-"`        // refs/tags/v0.0.1, refs/heads/master
	RefType  string `json:"ref_type"` // tag, branch
	RefName  string `json:"ref_name"`
	Branch   string `json:"-"`
	Tag      string `json:"-"`
	Owner    string `json:"repo_owner"`
	Repo     string `json:"repo_name"`
}

var Providers = make(map[string]func())
var Webhooks = make(map[string]func(chi.Router))

var Hooks = make(chan Ref)

func Hook(r Ref) {
	Hooks <- r
}

func Accept() Ref {
	return <-Hooks
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
