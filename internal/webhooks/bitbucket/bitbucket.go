package bitbucket

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"

	"github.com/go-chi/chi"
	"github.com/google/go-github/v32/github"
)

func init() {
	var secret string
	name := "bitbucket"
	options.ServerFlags.StringVar(
		&secret, fmt.Sprintf("%s-secret", name), "",
		fmt.Sprintf(
			"secret for %s webhooks (same as %s_SECRET=)",
			name, strings.ToUpper(name)),
	)
	webhooks.AddProvider("bitbucket", InitWebhook("bitbucket", &secret, "BITBUCKET_SECRET"))
}

// InitWebhook prepares the webhook router.
// It should be called after arguments are parsed and ENVs are set.InitWebhook
func InitWebhook(providername string, secret *string, envname string) func() {
	return func() {
		if "" == *secret {
			*secret = os.Getenv(envname)
		}
		if "" == *secret {
			fmt.Fprintf(os.Stderr, "skipped route for missing %s\n", envname)
			return
		}
		secretB := []byte(*secret)
		webhooks.AddRouteHandler(providername, func(router chi.Router) {
			router.Post("/", func(w http.ResponseWriter, r *http.Request) {
				r.Body = http.MaxBytesReader(w, r.Body, options.DefaultMaxBodySize)

				accessToken := r.URL.Query().Get("access_token")
				if "" != accessToken {
					if 0 == subtle.ConstantTimeCompare(
						[]byte(r.URL.Query().Get("access_token")),
						secretB,
					) {
						log.Printf("invalid bitbucket access_token\n")
						http.Error(w, "invalid access_token", http.StatusBadRequest)
						return
					}
				}

				payload, err := ioutil.ReadAll(r.Body)
				if err != nil {
					// if there's a read error, it should have been handled
					// already by the MaxBytesReader
					return
				}

				if "" == accessToken {
					sig := r.Header.Get("X-Hub-Signature")
					// TODO replace with generic X-Hub-Signature validation
					if err := github.ValidateSignature(sig, payload, secretB); nil != err {
						log.Printf("invalid bitbucket signature: error: %s\n", err)
						http.Error(w, "invalid bitbucket signature", http.StatusBadRequest)
						return
					}
				}

				info := Webhook{}
				if err := json.Unmarshal(payload, &info); nil != err {
					log.Printf("invalid bitbucket payload: error: %s\n%s\n", err, string(payload))
					http.Error(w, "invalid bitbucket payload", http.StatusBadRequest)
					return
				}

				var branch string
				var tag string
				var ref string

				n := len(info.Push.Changes)
				if n < 1 {
					log.Printf("invalid bitbucket changeset (n): %d\n%s\n", n, string(payload))
					http.Error(w, "invalid bitbucket payload", http.StatusBadRequest)
					return
				} else if n > 1 {
					log.Printf("more than one bitbucket changeset (n): %d\n%s\n", n, string(payload))
				}

				refName := info.Push.Changes[0].New.Name
				refType := info.Push.Changes[0].New.Type
				switch refType {
				case "tag":
					tag = refName
					ref = fmt.Sprintf("refs/tags/%s", refName)
				case "branch":
					branch = refName
					ref = fmt.Sprintf("refs/heads/%s", refName)
				default:
					log.Println("unexpected bitbucket RefType", refType)
					ref = fmt.Sprintf("refs/UNKNOWN/%s", refName)
				}

				switch refType {
				case "tags":
					refType = "tag"
					tag = refName
				case "heads":
					refType = "branch"
					branch = refName
				}

				var rev string
				if len(info.Push.Changes[0].Commits) > 0 {
					// TODO first or last?
					// TODO shouldn't tags have a Commit as well?
					rev = info.Push.Changes[0].Commits[0].Hash
				}

				webhooks.Hook(webhooks.Ref{
					HTTPSURL: info.Repository.Links.HTML.Href,
					Rev:      rev,
					Ref:      ref,
					RefType:  refType,
					RefName:  refName,
					Branch:   branch,
					Tag:      tag,
					Repo:     info.Repository.Name,
					Owner:    info.Repository.Workspace.Slug,
				})
			})
		})
	}
}
