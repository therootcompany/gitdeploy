// // +build github

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi"
	"github.com/google/go-github/v32/github"
)

func init() {
	githubSecret := ""
	runFlags.StringVar(&githubSecret, "github-secret", "", "secret for github webhooks (same as GITHUB_SECRET=)")
	webhookProviders["github"] = registerGithubish("github", &githubSecret, "GITHUB_SECRET")
}

func registerGithubish(providername string, secret *string, envname string) func() {
	return func() {
		if "" == *secret {
			*secret = os.Getenv(envname)
		}
		if "" == *secret {
			fmt.Fprintf(os.Stderr, "skipped route for missing %s\n", envname)
			return
		}
		githubSecretB := []byte(*secret)
		webhooks[providername] = func(router chi.Router) {
			router.Post("/", func(w http.ResponseWriter, r *http.Request) {
				body := http.MaxBytesReader(w, r.Body, maxBodySize)
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
					hooks <- webhook{
						rev:    e.GetAfter(), // *e.After
						ref:    ref,
						branch: branch,
						repo:   e.GetRepo().GetName(),         // *e.Repo.Name
						org:    e.GetRepo().GetOrganization(), // *e.Repo.Organization
					}
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
		}
	}
}
