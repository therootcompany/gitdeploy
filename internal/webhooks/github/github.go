package github

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"git.ryanburnette.com/ryanburnette/git-deploy/internal/options"
	"git.ryanburnette.com/ryanburnette/git-deploy/internal/webhooks"

	"github.com/go-chi/chi"
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
				body := http.MaxBytesReader(w, r.Body, options.DefaultMaxBodySize)
				defer func() {
					_ = body.Close()
				}()

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
					// this is a commit push, do something with it

					ref := e.GetRef() // *e.Ref
					branch := ref[len("refs/heads/"):]
					webhooks.Hook(webhooks.Ref{
						Rev:    e.GetAfter(), // *e.After
						Ref:    ref,
						Branch: branch,
						Repo:   e.GetRepo().GetName(),         // *e.Repo.Name
						Org:    e.GetRepo().GetOrganization(), // *e.Repo.Organization
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
