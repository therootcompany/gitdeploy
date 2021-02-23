package webhooks

import (
	"encoding/base64"
	"os"
	"strings"
	"time"

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
	RepoID    string    `json:"repo_id"`
	Timestamp time.Time `json:"timestamp"`
	HTTPSURL  string    `json:"https_url"`
	SSHURL    string    `json:"ssh_url"`
	Rev       string    `json:"rev"`
	Ref       string    `json:"ref"`      // refs/tags/v0.0.1, refs/heads/master
	RefType   string    `json:"ref_type"` // tag, branch
	RefName   string    `json:"ref_name"`
	Owner     string    `json:"repo_owner"`
	Repo      string    `json:"repo_name"`
	//Branch    string    `json:"branch"` // deprecated
	//Tag       string    `json:"tag"`    // deprecated
}

// RefID is a newtype string
type RefID string

// URLSafeRefID is a newtype string
type URLSafeRefID string

// RevID is a newtype string
type RevID string

// URLSafeRevID is a newtype string
type URLSafeRevID string

// URLSafeGitID is a newtype string
type URLSafeGitID string

// New returns a normalized Ref (Git reference)
func New(r Ref) *Ref {
	if len(r.HTTPSURL) > 0 {
		r.RepoID = getRepoID(r.HTTPSURL)
	} else /*if len(r.SSHURL) > 0*/ {
		r.RepoID = getRepoID(r.SSHURL)
	}
	r.Timestamp = getTimestamp(r.Timestamp)

	return &r
}

// String prints object as git.example.com#branch@rev
func (h *Ref) String() string {
	return string(h.GetRefID()) + "@" + h.Rev[:7]
}

// GetRefID returns a unique reference like "github.com/org/project#branch"
func (h *Ref) GetRefID() RefID {
	return RefID(h.RepoID + "#" + h.RefName)
}

// GetURLSafeRefID returns the URL-safe Base64 encoding of the RefID
func (h *Ref) GetURLSafeRefID() URLSafeRefID {
	return URLSafeRefID(
		base64.RawURLEncoding.EncodeToString(
			[]byte(h.GetRefID()),
		),
	)
}

// GetRevID returns a unique reference like "github.com/org/project#abcd7890"
func (h *Ref) GetRevID() RevID {
	return RevID(h.RepoID + "#" + h.Rev)
}

// GetURLSafeRevID returns the URL-safe Base64 encoding of the RevID
func (h *Ref) GetURLSafeRevID() URLSafeRevID {
	return URLSafeRevID(
		base64.RawURLEncoding.EncodeToString(
			[]byte(h.GetRevID()),
		),
	)
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

// https://git.example.com/example/project.git
//      => git.example.com/example/project
func getRepoID(url string) string {
	repoID := strings.TrimPrefix(url, "https://")
	repoID = strings.TrimPrefix(repoID, "http://")
	repoID = strings.TrimPrefix(repoID, "ssh://")
	repoID = strings.TrimSuffix(repoID, ".git")
	return repoID
}

func getTimestamp(t time.Time) time.Time {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t
}
