package github

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"

	"github.com/go-chi/chi"
	// TODO nix this dependency in favor of a lightweight X-Hub-Signature
	// and JSON-to-Go-struct approach
	"github.com/google/go-github/v33/github"
)

func init() {
	var githubSecrets string
	options.ServerFlags.StringVar(
		&githubSecrets, "github-secret", "",
		"secret for github webhooks (same as GITHUB_SECRET=)",
	)
	webhooks.AddProvider("github", InitWebhook("github", &githubSecrets, "GITHUB_SECRET"))
}

// InitWebhook initializes the webhook when registered
func InitWebhook(providername string, secretList *string, envname string) func() {
	return func() {
		secrets := webhooks.ParseSecrets(providername, *secretList, envname)
		if 0 == len(secrets) {
			fmt.Fprintf(os.Stderr, "skipped route for missing %q\n", envname)
			return
		}

		webhooks.AddRouteHandler(providername, func(router chi.Router) {
			router.Post("/", func(w http.ResponseWriter, r *http.Request) {
				r.Body = http.MaxBytesReader(w, r.Body, options.DefaultMaxBodySize)

				payload, err := ioutil.ReadAll(r.Body)
				if err != nil {
					// if there's a read error, it should have been handled already by the MaxBytesReader
					return
				}

				sig := r.Header.Get("X-Hub-Signature")
				for _, secret := range secrets {
					if err = github.ValidateSignature(sig, payload, secret); nil != err {
						continue
					}
					// err = nil
					break
				}
				if nil != err {
					log.Printf("invalid %q signature: error: %s\n", providername, err)
					http.Error(w, fmt.Sprintf("invalid %q signature", providername), http.StatusBadRequest)
					return
				}

				hookType := github.WebHookType(r)
				event, err := github.ParseWebHook(hookType, payload)
				if err != nil {
					log.Printf("invalid github webhook payload: error: %s\n", err)
					http.Error(w, "invalid github webhook payload", http.StatusBadRequest)
					return
				}

				switch e := event.(type) {
				case *github.PushEvent:
					//var branch string
					//var tag string

					ref := e.GetRef() // *e.Ref
					parts := strings.Split(ref, "/")
					refType := parts[1]
					prefixLen := len("refs/") + len(refType) + len("/")
					refName := ref[prefixLen:]
					switch refType {
					case "tags":
						refType = "tag"
						//tag = refName
					case "heads":
						refType = "branch"
						//branch = refName
					}

					webhooks.Hook(webhooks.Ref{
						Timestamp: e.GetRepo().GetPushedAt().Time,
						HTTPSURL:  e.GetRepo().GetCloneURL(),
						SSHURL:    e.GetRepo().GetSSHURL(),
						Rev:       e.GetAfter(), // *e.After
						Ref:       ref,
						RefType:   refType,
						RefName:   refName,
						Repo:      e.GetRepo().GetName(), // *e.Repo.Name
						Owner:     e.GetRepo().GetOwner().GetLogin(),
						//Branch:    branch,
						//Tag:       tag,
					})
				/*
					case *github.PullRequestEvent:
						// probably doesn't matter
					case *github.StatusEvent:
						// probably doesn't matter
					case *github.WatchEvent:
						// probably doesn't matter
				*/
				default:
					log.Printf("unknown event type %s\n", hookType)
					return
				}

			})
		})
	}
}
