package github

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"git.ryanburnette.com/ryanburnette/git-deploy/internal/options"
	"git.ryanburnette.com/ryanburnette/git-deploy/internal/webhooks"

	"github.com/go-chi/chi"
	// TODO nix this dependency in favor of a lightweight X-Hub-Signature
	// and JSON-to-Go-struct approach
	"github.com/google/go-github/v32/github"
)

func init() {
	var githubSecret string
	options.ServerFlags.StringVar(
		&githubSecret, "github-secret", "",
		"secret for github webhooks (same as GITHUB_SECRET=)",
	)
	webhooks.AddProvider("github", InitWebhook("github", &githubSecret, "GITHUB_SECRET"))
}

func InitWebhook(providername string, secret *string, envname string) func() {
	return func() {
		if "" == *secret {
			*secret = os.Getenv(envname)
		}
		if "" == *secret {
			fmt.Fprintf(os.Stderr, "skipped route for missing %s\n", envname)
			return
		}
		githubSecretB := []byte(*secret)
		webhooks.AddRouteHandler(providername, func(router chi.Router) {
			router.Post("/", func(w http.ResponseWriter, r *http.Request) {
				r.Body = http.MaxBytesReader(w, r.Body, options.DefaultMaxBodySize)

				payload, err := ioutil.ReadAll(r.Body)
				if err != nil {
					// if there's a read error, it should have been handled already by the MaxBytesReader
					return
				}

				sig := r.Header.Get("X-Hub-Signature")
				if err := github.ValidateSignature(sig, payload, githubSecretB); nil != err {
					log.Printf("invalid github signature: error: %s\n", err)
					http.Error(w, "invalid github signature", http.StatusBadRequest)
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
					var branch string
					var tag string

					ref := e.GetRef() // *e.Ref
					parts := strings.Split(ref, "/")
					refType := parts[1]
					prefixLen := len("refs/") + len(refType) + len("/")
					refName := ref[prefixLen:]
					switch refType {
					case "tags":
						refType = "tag"
						tag = refName
					case "heads":
						refType = "branch"
						branch = refName
					}
					webhooks.Hook(webhooks.Ref{
						HTTPSURL: e.GetRepo().GetCloneURL(),
						SSHURL:   e.GetRepo().GetSSHURL(),
						Rev:      e.GetAfter(), // *e.After
						Ref:      ref,
						RefType:  refType,
						RefName:  refName,
						Branch:   branch,
						Tag:      tag,
						Repo:     e.GetRepo().GetName(), // *e.Repo.Name
						Owner:    e.GetRepo().GetOwner().GetLogin(),
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
