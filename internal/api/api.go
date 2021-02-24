package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"git.rootprojects.org/root/gitdeploy/internal/jobs"
	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"

	"github.com/go-chi/chi"
)

type HTTPError struct {
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type Report struct {
	Report *jobs.Report `json:"report"`
}

// Route will set up the API and such
func Route(r chi.Router, runOpts *options.ServerConfig) {
	jobs.Start(runOpts)
	RouteStopped(r, runOpts)
}

// RouteStopped is for testing
func RouteStopped(r chi.Router, runOpts *options.ServerConfig) {
	webhooks.RouteHandlers(r)

	r.Route("/api", func(r chi.Router) {

		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// r.Body is always .Close()ed by Go's http server
				r.Body = http.MaxBytesReader(w, r.Body, options.DefaultMaxBodySize)
				// TODO admin auth middleware
				log.Printf("TODO: handle authentication")
				next.ServeHTTP(w, r)
			})
		})

		r.Route("/admin", func(r chi.Router) {
			r.Get("/repos", func(w http.ResponseWriter, r *http.Request) {
				repos := []Repo{}

				for _, id := range strings.Fields(runOpts.RepoList) {
					repos = append(repos, Repo{
						ID:         id,
						CloneURL:   fmt.Sprintf("https://%s.git", id),
						Promotions: runOpts.Promotions,
					})
				}
				err := filepath.Walk(runOpts.ScriptsPath, func(path string, info os.FileInfo, err error) error {
					if nil != err {
						fmt.Printf("error walking %q: %v\n", path, err)
						return nil
					}
					// "scripts/github.com/org/repo"
					parts := strings.Split(filepath.ToSlash(path), "/")
					if len(parts) < 3 {
						return nil
					}
					path = strings.Join(parts[1:], "/")
					if info.Mode().IsRegular() && "deploy.sh" == info.Name() && runOpts.ScriptsPath != path {
						id := filepath.Dir(path)
						repos = append(repos, Repo{
							ID:         id,
							CloneURL:   fmt.Sprintf("https://%s.git", id),
							Promotions: runOpts.Promotions,
						})
					}
					return nil
				})
				if nil != err {
					http.Error(w, "the scripts directory disappeared", http.StatusInternalServerError)
					return
				}
				b, _ := json.MarshalIndent(ReposResponse{
					Success: true,
					Repos:   repos,
				}, "", "  ")
				w.Header().Set("Content-Type", "application/json")
				w.Write(append(b, '\n'))
			})

			r.Get("/logs/{oldID}", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				oldID := webhooks.URLSafeGitID(chi.URLParam(r, "oldID"))
				// TODO add `since`
				j, err := jobs.LoadLogs(runOpts, oldID)
				if nil != err {
					w.WriteHeader(404)
					w.Write([]byte(
						`{ "success": false, "error": "job log does not exist" }` + "\n",
					))
					return
				}

				b, _ := json.MarshalIndent(struct {
					Success bool `json:"success"`
					jobs.Job
				}{
					Success: true,
					Job:     *j,
				}, "", "  ")
				w.Write(append(b, '\n'))
			})

			/*
				r.Get("/logs/*", func(w http.ResponseWriter, r *http.Request) {
					// TODO add ?since=
					// TODO JSON logs
					logPath := chi.URLParam(r, "*")
					f, err := os.Open(filepath.Join(os.Getenv("LOG_DIR"), logPath))
					if nil != err {
						w.WriteHeader(404)
						w.Write([]byte(
							`{ "success": false, "error": "job log does not exist" }` + "\n",
						))
						return
					}
					io.Copy(w, f)
				})
			*/

			r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
				all := jobs.All()

				b, _ := json.Marshal(struct {
					Success bool        `json:"success"`
					Jobs    []*jobs.Job `json:"jobs"`
				}{
					Success: true,
					Jobs:    all,
				})
				w.Write(append(b, '\n'))
			})

			r.Post("/jobs", func(w http.ResponseWriter, r *http.Request) {
				decoder := json.NewDecoder(r.Body)
				msg := &jobs.KillMsg{}
				if err := decoder.Decode(msg); nil != err {
					log.Printf("kill job invalid json:\n%v", err)
					http.Error(w, "invalid json body", http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				if _, ok := jobs.Actives.Load(webhooks.URLSafeRefID(msg.JobID)); !ok {
					if _, ok := jobs.Pending.Load(webhooks.URLSafeRefID(msg.JobID)); !ok {
						w.Write([]byte(
							`{ "success": false, "error": "job does not exist" }` + "\n",
						))
						return
					}
				}

				// killing a job *should* always succeed ...right?
				jobs.Remove(webhooks.URLSafeRefID(msg.JobID))
				w.Write([]byte(
					`{ "success": true }` + "\n",
				))
			})

			r.Post("/promote", func(w http.ResponseWriter, r *http.Request) {
				decoder := json.NewDecoder(r.Body)
				msg := webhooks.Ref{}
				if err := decoder.Decode(&msg); nil != err {
					log.Printf("promotion job invalid json:\n%v", err)
					http.Error(w, "invalid json body", http.StatusBadRequest)
					return
				}
				if "" == msg.HTTPSURL || "" == msg.RefName {
					log.Printf("promotion job incomplete json %s", msg)
					http.Error(w, "incomplete json body", http.StatusBadRequest)
					return
				}

				n := -2
				for i := range runOpts.Promotions {
					if runOpts.Promotions[i] == msg.RefName {
						n = i - 1
						break
					}
				}
				if n < 0 {
					log.Printf("promotion job invalid: cannot promote: %d", n)
					http.Error(w, "invalid promotion", http.StatusBadRequest)
					return
				}

				promoteTo := runOpts.Promotions[n]
				jobs.Promote(msg, promoteTo)

				b, _ := json.Marshal(struct {
					Success   bool   `json:"success"`
					PromoteTo string `json:"promote_to"`
				}{
					Success:   true,
					PromoteTo: promoteTo,
				})
				w.Write(append(b, '\n'))

			})
		})

		r.Route("/local", func(r chi.Router) {

			r.Post("/jobs/{jobID}", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				decoder := json.NewDecoder(r.Body)
				report := &Report{}
				if err := decoder.Decode(report); nil != err {
					w.WriteHeader(http.StatusBadRequest)
					writeError(w, HTTPError{
						Code:    "E_PARSE",
						Message: "could not parse request body",
						Detail:  err.Error(),
					})
					return
				}

				jobID := webhooks.URLSafeRefID(chi.URLParam(r, "jobID"))
				if err := jobs.SetReport(jobID, report.Report); nil != err {
					w.WriteHeader(http.StatusInternalServerError)
					writeError(w, HTTPError{
						Code:    "E_SERVER",
						Message: "could not update report",
						Detail:  err.Error(),
					})
					return
				}

				// Attach additional logs / reports to running job
				w.Write([]byte(`{ "success": true }` + "\n"))
			})

		})

	})

}

func writeError(w http.ResponseWriter, err HTTPError) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(err)
	w.Write([]byte("\n"))
}

// ReposResponse is the successful response to /api/repos
type ReposResponse struct {
	Success bool   `json:"success"`
	Repos   []Repo `json:"repos"`
}

// Repo is one of the elements of /api/repos
type Repo struct {
	ID         string   `json:"id"`
	CloneURL   string   `json:"clone_url"`
	Promotions []string `json:"_promotions"`
}
