package main

import (
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"git.rootprojects.org/root/gitdeploy/assets"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/joho/godotenv/autoload"
)

var (
	name    = "gitdeploy"
	version = "0.0.0"
	date    = "0001-01-01T00:00:00Z"
	commit  = "0000000"
)

func usage() {
	ver()
	fmt.Println("")
	fmt.Println("Use 'help <command>'")
	fmt.Println("  help")
	fmt.Println("  init")
	fmt.Println("  run")
}

func ver() {
	fmt.Printf("%s v%s %s (%s)\n", name, version, commit[:7], date)
}

type job struct {
	ID        string // {HTTPSURL}#{BRANCH}
	Cmd       *exec.Cmd
	Ref       webhooks.Ref
	CreatedAt time.Time
}

var jobs = make(map[string]*job)
var killers = make(chan string)

var runOpts *options.ServerConfig
var runFlags *flag.FlagSet
var initFlags *flag.FlagSet
var promotions []string
var promotionList string
var defaultPromotionList = "production,staging,master"

func init() {
	runOpts = options.Server
	runFlags = options.ServerFlags
	initFlags = options.InitFlags
	runFlags.StringVar(&runOpts.Addr, "listen", "", "the address and port on which to listen (default :4483)")
	runFlags.BoolVar(&runOpts.TrustProxy, "trust-proxy", false, "trust X-Forwarded-For header")
	runFlags.BoolVar(&runOpts.Compress, "compress", true, "enable compression for text,html,js,css,etc")
	runFlags.StringVar(
		&runOpts.ServePath, "serve-path", "",
		"path to serve, falls back to built-in web app")
	runFlags.StringVar(
		&runOpts.Exec, "exec", "",
		"path to ./scripts/{deploy.sh,promote.sh,etc}")
	//"path to bash script to run with git info as arguments")
	runFlags.StringVar(&promotionList, "promotions", "",
		"a list of promotable branches in descending order (default '"+defaultPromotionList+"')")
}

func main() {
	args := os.Args[:]
	if 1 == len(args) {
		// "run" should be the default
		args = append(args, "run")
	}

	if "help" == args[1] {
		// top-level help
		if 2 == len(args) {
			usage()
			os.Exit(0)
			return
		}
		// move help to subcommand argument
		self := args[0]
		args = append([]string{self}, args[2:]...)
		args = append(args, "--help")
	}

	switch args[1] {
	case "version":
		ver()
		os.Exit(0)
		return
	case "init":
		_ = initFlags.Parse(args[2:])
	case "run":
		_ = runFlags.Parse(args[2:])
		if "" == runOpts.Exec {
			fmt.Printf("--exec <path/to/scripts/> is a required flag")
			os.Exit(1)
			return
		}
		if 0 == len(runOpts.Addr) {
			runOpts.Addr = os.Getenv("LISTEN")
		}
		if 0 == len(runOpts.Addr) {
			runOpts.Addr = "localhost:" + os.Getenv("PORT")
		}
		if 0 == len(runOpts.Addr) {
			fmt.Printf("--listen <[addr]:port> is a required flag")
			os.Exit(1)
			return
		}
		if 0 == len(promotionList) {
			promotionList = os.Getenv("PROMOTIONS")
		}
		if 0 == len(promotionList) {
			promotionList = defaultPromotionList
		}
		promotions = strings.Fields(
			strings.ReplaceAll(promotionList, ",", " "),
		)

		webhooks.MustRegisterAll()
		serve()
	default:
		usage()
		os.Exit(1)
		return
	}
}

// Job is the JSON we send back through the API about jobs
type Job struct {
	JobID     string       `json:"job_id"`
	CreatedAt time.Time    `json:"created_at"`
	Ref       webhooks.Ref `json:"ref"`
	Promote   bool         `json:"promote,omitempty"`
}

// KillMsg describes which job to kill
type KillMsg struct {
	JobID string `json:"job_id"`
	Kill  bool   `json:"kill"`
}

func serve() {
	r := chi.NewRouter()

	// A good base middleware stack
	if runOpts.TrustProxy {
		r.Use(middleware.RealIP)
	}
	if runOpts.Compress {
		r.Use(middleware.Compress(flate.DefaultCompression))
	}
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Recoverer)

	var staticHandler http.HandlerFunc
	pub := http.FileServer(assets.Assets)

	if len(runOpts.ServePath) > 0 {
		// try the user-provided directory first, then fallback to the built-in
		devFS := http.Dir(runOpts.ServePath)
		dev := http.FileServer(devFS)
		staticHandler = func(w http.ResponseWriter, r *http.Request) {
			if _, err := devFS.Open(r.URL.Path); nil != err {
				pub.ServeHTTP(w, r)
				return
			}
			dev.ServeHTTP(w, r)
		}
	} else {
		staticHandler = func(w http.ResponseWriter, r *http.Request) {
			pub.ServeHTTP(w, r)
		}
	}

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

		r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
			// again, possible race condition, but not one that much matters
			jjobs := []Job{}
			for jobID, job := range jobs {
				jjobs = append(jjobs, Job{
					JobID:     jobID,
					Ref:       job.Ref,
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
			for i := range promotions {
				if promotions[i] == msg.RefName {
					n = i - 1
					break
				}
			}
			if n < 0 {
				log.Println("promotion job invalid: cannot promote:", n)
				http.Error(w, "invalid promotion", http.StatusBadRequest)
				return
			}

			promoteTo := promotions[n]
			runPromote(*msg, promoteTo)

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
	r.Get("/*", staticHandler)

	fmt.Println("Listening for http (with reasonable timeouts) on", runOpts.Addr)
	srv := &http.Server{
		Addr:              runOpts.Addr,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		MaxHeaderBytes:    1024 * 1024, // 1MiB
	}

	go func() {
		// TODO read from backlog
		for {
			//hook := webhooks.Accept()
			select {
			case hook := <-webhooks.Hooks:
				runHook(hook)
			case jobID := <-killers:
				remove(jobID, false)
			}
		}
	}()

	if err := srv.ListenAndServe(); nil != err {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
		return
	}
}

func runHook(hook webhooks.Ref) {
	fmt.Printf("%#v\n", hook)
	jobID := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.HTTPSURL, hook.RefName),
	))

	args := []string{
		runOpts.Exec + "/deploy.sh",
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

	env := os.Environ()
	envs := []string{
		"GIT_DEPLOY_JOB_ID=" + jobID,
		"GIT_REF_NAME=" + hook.RefName,
		"GIT_REF_TYPE=" + hook.RefType,
		"GIT_REPO_ID=" + repoID,
		"GIT_REPO_OWNER=" + hook.Owner,
		"GIT_REPO_NAME=" + hook.Repo,
		"GIT_CLONE_URL=" + hook.HTTPSURL,
	}
	cmd.Env = append(env, envs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if _, exists := jobs[jobID]; exists {
		// TODO put job in backlog
		log.Printf("gitdeploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
		return
	}

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	jobs[jobID] = &job{
		ID:        jobID,
		Cmd:       cmd,
		Ref:       hook,
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
		// TODO check for backlog
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

func runPromote(hook webhooks.Ref, promoteTo string) {
	// TODO create an origin-branch tag with a timestamp?
	jobID1 := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.HTTPSURL, hook.RefName),
	))
	jobID2 := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.HTTPSURL, promoteTo),
	))

	args := []string{
		runOpts.Exec + "/promote.sh",
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
		log.Printf("gitdeploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
		return
	}
	if _, exists := jobs[jobID2]; exists {
		// TODO put promote in backlog
		log.Printf("gitdeploy job already started for %s#%s\n", hook.HTTPSURL, promoteTo)
		return
	}

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	jobs[jobID1] = &job{
		ID:        jobID2,
		Cmd:       cmd,
		Ref:       hook,
		CreatedAt: time.Now(),
	}
	jobs[jobID2] = &job{
		ID:        jobID2,
		Cmd:       cmd,
		Ref:       hook,
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
