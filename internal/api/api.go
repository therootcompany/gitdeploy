package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"

	"github.com/go-chi/chi"
)

// HookResponse is a GitRef but with a little extra as HTTP response
type HookResponse struct {
	RepoID    string    `json:"repo_id"`
	CreatedAt time.Time `json:"created_at"`
	EndedAt   time.Time `json:"ended_at"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	Log       string    `json:"log"`
	LogURL    string    `json:"log_url"`
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

// Route will set up the API and such
func Route(r chi.Router, runOpts *options.ServerConfig) {
	LogDir = os.Getenv("LOG_DIR")

	go func() {
		// TODO read from backlog
		for {
			//hook := webhooks.Accept()
			select {
			case promotion := <-Promotions:
				gitref := webhooks.New(*promotion.Ref)
				runPromote(*gitref, promotion.PromoteTo, runOpts)
			case hook := <-webhooks.Hooks:
				normalizeHook(&hook)
				debounceHook(hook, runOpts)
			case jobID := <-killers:
				// should !nokill (so... should kill job on the spot?)
				removeJob(jobID /*, false*/)
			}
		}
	}()

	go func() {
		for {
			time.Sleep(time.Minute)

			// TODO mutex here again (or switch to sync.Map)
			staleJobIDs := []string{}
			for jobID, job := range recentJobs {
				if time.Now().Sub(job.CreatedAt) > time.Hour {
					staleJobIDs = append(staleJobIDs, jobID)
				}
			}
			// and here
			for i := range staleJobIDs {
				delete(recentJobs, staleJobIDs[i])
			}
		}
	}()

	webhooks.RouteHandlers(r)

	r.Route("/api/admin", func(r chi.Router) {

		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// r.Body is always .Close()ed by Go's http server
				r.Body = http.MaxBytesReader(w, r.Body, options.DefaultMaxBodySize)
				// TODO admin auth middleware
				log.Printf("TODO: handle authentication")
				next.ServeHTTP(w, r)
			})
		})

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

		r.Get("/logs", func(w http.ResponseWriter, r *http.Request) {
			// TODO add `since`
			logs := []HookResponse{}

			// Walk LOG_DIR
			// group by LOG_DIR/**/*.log
			// TODO delete old logs 15d+?
			hooks, _ := WalkLogs(os.Getenv("LOG_DIR"))
			for _, hook := range hooks {
				//fmt.Printf("%#v\n\n", hook)
				logName := hook.Timestamp.Format(TimeFile) + "." + hook.RefName + "." + hook.Rev
				logURL := "/logs/" + hook.RepoID + "/" + logName
				logStat, _ := os.Stat(filepath.Join("LOG_DIR", hook.RepoID, logName+".log"))
				exitCode := 255
				logs = append(logs, HookResponse{
					RepoID:    hook.RepoID,
					CreatedAt: hook.Timestamp,
					EndedAt:   logStat.ModTime(),
					ExitCode:  &exitCode,
					Log:       logName,
					LogURL:    logURL,
				})
			}

			// join with active jobs
			for _, job := range jobs {
				hook := job.GitRef
				//fmt.Printf("%#v\n\n", hook)
				logName := hook.Timestamp.Format(TimeFile) + "." + hook.RefName + "." + hook.Rev
				logURL := "/logs/" + hook.RepoID + "/" + logName
				logs = append(logs, HookResponse{
					RepoID:    hook.RepoID,
					CreatedAt: hook.Timestamp,
					//EndedAt: ,
					ExitCode: job.ExitCode,
					Log:      logName,
					LogURL:   logURL,
				})
			}

			b, _ := json.Marshal(struct {
				Success bool `json:"success"`
				Logs    []HookResponse
			}{
				Success: true,
				Logs:    logs,
			})
			w.Write(append(b, '\n'))
		})

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

		r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
			// again, possible race condition, but not one that much matters
			jobsCopy := []Job{}
			for jobID, job := range jobs {
				jobsCopy = append(jobsCopy, Job{
					JobID:     jobID,
					GitRef:    job.GitRef,
					CreatedAt: job.CreatedAt,
				})
			}
			// and here too
			for jobID, job := range recentJobs {
				jobsCopy = append(jobsCopy, Job{
					JobID:     jobID,
					GitRef:    job.GitRef,
					CreatedAt: job.CreatedAt,
				})
			}

			b, _ := json.Marshal(struct {
				Success bool  `json:"success"`
				Jobs    []Job `json:"jobs"`
			}{
				Success: true,
				Jobs:    jobsCopy,
			})
			w.Write(append(b, '\n'))
		})

		r.Post("/jobs", func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				_ = r.Body.Close()
			}()

			decoder := json.NewDecoder(r.Body)
			msg := &KillMsg{}
			if err := decoder.Decode(msg); nil != err {
				log.Printf("kill job invalid json:\n%v", err)
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			// possible race condition, but not the kind that should matter
			if _, exists := jobs[msg.JobID]; !exists {
				w.Write([]byte(
					`{ "success": false, "error": "job does not exist" }` + "\n",
				))
				return
			}

			// killing a job *should* always succeed ...right?
			killers <- msg.JobID
			w.Write([]byte(
				`{ "success": true }` + "\n",
			))
		})

		r.Post("/jobs/{jobID}", func(w http.ResponseWriter, r *http.Request) {
			// Attach additional logs / reports to running job
			w.Write([]byte(
				`{ "success": true }` + "\n",
			))
		})

		r.Post("/promote", func(w http.ResponseWriter, r *http.Request) {
			decoder := json.NewDecoder(r.Body)
			msg := &webhooks.Ref{}
			if err := decoder.Decode(msg); nil != err {
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
			Promotions <- Promotion{
				PromoteTo: promoteTo,
				Ref:       msg,
			}

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

}
