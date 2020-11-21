package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"

	"github.com/go-chi/chi"
)

type job struct {
	ID        string // {HTTPSURL}#{BRANCH}
	Cmd       *exec.Cmd
	GitRef    webhooks.Ref
	CreatedAt time.Time
}

var jobs = make(map[string]*job)
var killers = make(chan string)
var tmpDir string

// Job is the JSON we send back through the API about jobs
type Job struct {
	JobID     string       `json:"job_id"`
	CreatedAt time.Time    `json:"created_at"`
	GitRef    webhooks.Ref `json:"ref"`
	Promote   bool         `json:"promote,omitempty"`
}

// KillMsg describes which job to kill
type KillMsg struct {
	JobID string `json:"job_id"`
	Kill  bool   `json:"kill"`
}

func init() {
	var err error
	tmpDir, err = ioutil.TempDir("", "gitdeploy-*")
	if nil != err {
		fmt.Fprintf(os.Stderr, "could not create temporary directory")
		os.Exit(1)
		return
	}
	log.Printf("TEMP_DIR=%s", tmpDir)
}

// Route will set up the API and such
func Route(r chi.Router, runOpts *options.ServerConfig) {

	go func() {
		// TODO read from backlog
		for {
			//hook := webhooks.Accept()
			select {
			case hook := <-webhooks.Hooks:
				runHook(hook, runOpts)
			case jobID := <-killers:
				remove(jobID, false)
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
				log.Println("TODO: handle authentication")
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
		r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
			// again, possible race condition, but not one that much matters
			jjobs := []Job{}
			for jobID, job := range jobs {
				jjobs = append(jjobs, Job{
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
				Jobs:    jjobs,
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
				log.Println("kill job invalid json:", err)
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

		r.Post("/promote", func(w http.ResponseWriter, r *http.Request) {
			decoder := json.NewDecoder(r.Body)
			msg := &webhooks.Ref{}
			if err := decoder.Decode(msg); nil != err {
				log.Println("promotion job invalid json:", err)
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}
			if "" == msg.HTTPSURL || "" == msg.RefName {
				log.Println("promotion job incomplete json", msg)
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
				log.Println("promotion job invalid: cannot promote:", n)
				http.Error(w, "invalid promotion", http.StatusBadRequest)
				return
			}

			promoteTo := runOpts.Promotions[n]
			runPromote(*msg, promoteTo, runOpts)

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

func runHook(hook webhooks.Ref, runOpts *options.ServerConfig) {
	fmt.Printf("%#v\n", hook)
	jobID := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.HTTPSURL, hook.RefName),
	))

	args := []string{
		runOpts.ScriptsPath + "/deploy.sh",
		jobID,
		hook.RefName,
		hook.RefType,
		hook.Owner,
		hook.Repo,
		hook.HTTPSURL,
	}
	cmd := exec.Command("bash", args...)

	// https://git.example.com/example/project.git
	//      => git.example.com/example/project
	repoID := strings.TrimPrefix(hook.HTTPSURL, "https://")
	repoID = strings.TrimPrefix(repoID, "https://")
	repoID = strings.TrimSuffix(repoID, ".git")
	jobName := fmt.Sprintf("%s#%s", strings.ReplaceAll(repoID, "/", "-"), hook.RefName)

	env := os.Environ()
	envs := []string{
		"GIT_DEPLOY_JOB_ID=" + jobID,
		"GIT_REF_NAME=" + hook.RefName,
		"GIT_REF_TYPE=" + hook.RefType,
		"GIT_REPO_ID=" + repoID,
		"GIT_REPO_OWNER=" + hook.Owner,
		"GIT_REPO_NAME=" + hook.Repo,
		"GIT_CLONE_URL=" + hook.HTTPSURL, // deprecated
		"GIT_HTTPS_URL=" + hook.HTTPSURL,
		"GIT_SSH_URL=" + hook.SSHURL,
	}
	for _, repo := range strings.Fields(runOpts.RepoList) {
		last := len(repo) - 1
		if len(repo) < 0 {
			continue
		}
		repoID = strings.ToLower(repoID)
		if '*' == repo[last] {
			// Wildcard match a prefix, for example:
			// github.com/whatever/*					MATCHES github.com/whatever/foo
			// github.com/whatever/ProjectX-* MATCHES github.com/whatever/ProjectX-Foo
			if strings.HasPrefix(repoID, repo[:last]) {
				envs = append(envs, "GIT_REPO_TRUSTED=true")
				break
			}
		} else if repo == repoID {
			envs = append(envs, "GIT_REPO_TRUSTED=true")
			break
		}
	}
	cmd.Env = append(env, envs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if _, exists := jobs[jobID]; exists {
		saveBacklog(hook, jobName, jobID)
		log.Printf("[runHook] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
		return
	}

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	jobs[jobID] = &job{
		ID:        jobID,
		Cmd:       cmd,
		GitRef:    hook,
		CreatedAt: time.Now(),
	}

	go func() {
		log.Printf("gitdeploy job for %s#%s started\n", hook.HTTPSURL, hook.RefName)
		if err := cmd.Wait(); nil != err {
			log.Printf("gitdeploy job for %s#%s exited with error: %v", hook.HTTPSURL, hook.RefName, err)
		} else {
			log.Printf("gitdeploy job for %s#%s finished\n", hook.HTTPSURL, hook.RefName)
		}
		remove(jobID, true)
		restoreBacklog(jobName, jobID)
	}()
}

func remove(jobID string, nokill bool) {
	job, exists := jobs[jobID]
	if !exists {
		return
	}
	delete(jobs, jobID)

	if nil != job.Cmd.ProcessState {
		// is not yet finished
		if nil != job.Cmd.Process {
			// but definitely was started
			err := job.Cmd.Process.Kill()
			log.Println("error killing job:", err)
		}
	}
}

func runPromote(hook webhooks.Ref, promoteTo string, runOpts *options.ServerConfig) {
	// TODO create an origin-branch tag with a timestamp?
	jobID1 := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.HTTPSURL, hook.RefName),
	))
	jobID2 := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.HTTPSURL, promoteTo),
	))

	args := []string{
		runOpts.ScriptsPath + "/promote.sh",
		jobID1,
		promoteTo,
		hook.RefName,
		hook.RefType,
		hook.Owner,
		hook.Repo,
		hook.HTTPSURL,
	}
	cmd := exec.Command("bash", args...)

	env := os.Environ()
	envs := []string{
		"GIT_DEPLOY_JOB_ID=" + jobID1,
		"GIT_DEPLOY_PROMOTE_TO=" + promoteTo,
		"GIT_REF_NAME=" + hook.RefName,
		"GIT_REF_TYPE=" + hook.RefType,
		"GIT_REPO_OWNER=" + hook.Owner,
		"GIT_REPO_NAME=" + hook.Repo,
		"GIT_CLONE_URL=" + hook.HTTPSURL,
	}
	cmd.Env = append(env, envs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if _, exists := jobs[jobID1]; exists {
		// TODO put promote in backlog
		log.Printf("[promote] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
		return
	}
	if _, exists := jobs[jobID2]; exists {
		// TODO put promote in backlog
		log.Printf("[promote] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, promoteTo)
		return
	}

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	jobs[jobID1] = &job{
		ID:        jobID2,
		Cmd:       cmd,
		GitRef:    hook,
		CreatedAt: time.Now(),
	}
	jobs[jobID2] = &job{
		ID:        jobID2,
		Cmd:       cmd,
		GitRef:    hook,
		CreatedAt: time.Now(),
	}

	go func() {
		log.Printf("gitdeploy promote for %s#%s started\n", hook.HTTPSURL, hook.RefName)
		_ = cmd.Wait()
		killers <- jobID1
		killers <- jobID2
		log.Printf("gitdeploy promote for %s#%s finished\n", hook.HTTPSURL, hook.RefName)
		// TODO check for backlog
	}()
}

func saveBacklog(hook webhooks.Ref, jobName, jobID string) {
	b, _ := json.MarshalIndent(hook, "", "  ")
	f, err := ioutil.TempFile(tmpDir, "tmp-*")
	if nil != err {
		log.Printf("[warn] could not create backlog file for %s:\n%v", jobID, err)
		return
	}
	if _, err := f.Write(b); nil != err {
		log.Printf("[warn] could not write backlog file for %s:\n%v", jobID, err)
		return
	}

	jobFile := filepath.Join(tmpDir, jobName)
	_ = os.Remove(jobFile)
	if err := os.Rename(f.Name(), jobFile); nil != err {
		log.Printf("[warn] could not rename file %s => %s:\n%v", f.Name(), jobFile, err)
		return
	}
	log.Printf("[BACKLOG] new backlog job for %s", jobName)
}

func restoreBacklog(jobName, jobID string) {
	jobFile := filepath.Join(tmpDir, jobName)
	_ = os.Remove(jobFile + ".cur")
	_ = os.Rename(jobFile, jobFile+".cur")

	b, err := ioutil.ReadFile(jobFile + ".cur")
	if nil != err {
		if !os.IsNotExist(err) {
			log.Printf("[warn] could not create backlog file for %s:\n%v", jobID, err)
		}
		// doesn't exist => no backlog
		log.Printf("[NO BACKLOG] no backlog items for %s", jobName)
		return
	}

	ref := webhooks.Ref{}
	if err := json.Unmarshal(b, &ref); nil != err {
		log.Printf("[warn] could not parse backlog file for %s:\n%v", jobID, err)
		return
	}
	log.Printf("[BACKLOG] pop backlog for %s", jobName)
	webhooks.Hook(ref)
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
