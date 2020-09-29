package main

import (
	"compress/flate"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"git.ryanburnette.com/ryanburnette/git-deploy/assets"
	"git.ryanburnette.com/ryanburnette/git-deploy/internal/options"
	"git.ryanburnette.com/ryanburnette/git-deploy/internal/webhooks"

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
	ID  string // {HTTPSURL}#{BRANCH}
	Cmd *exec.Cmd
	Ref webhooks.Ref
}

var jobs = make(map[string]*job)

var runOpts *options.ServerConfig
var runFlags *flag.FlagSet
var initFlags *flag.FlagSet

func init() {
	runOpts = options.Server
	runFlags = options.ServerFlags
	initFlags = options.InitFlags
	runFlags.StringVar(&runOpts.Addr, "listen", ":3000", "the address and port on which to listen")
	runFlags.BoolVar(&runOpts.TrustProxy, "trust-proxy", false, "trust X-Forwarded-For header")
	runFlags.BoolVar(&runOpts.Compress, "compress", true, "enable compression for text,html,js,css,etc")
	runFlags.StringVar(
		&runOpts.ServePath, "serve-path", "",
		"path to serve, falls back to built-in web app")
	runFlags.StringVar(
		&runOpts.Exec, "exec", "",
		"path to bash script to run with git info as arguments")
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
			fmt.Printf("--exec <path/to/script.sh> is a required flag")
			os.Exit(1)
			return
		}
		webhooks.MustRegisterAll()
		serve()
	default:
		usage()
		os.Exit(1)
		return
	}
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
		for {
			hook := webhooks.Accept()
			// TODO os.Exec
			fmt.Printf("%#v\n", hook)
			jobID := base64.URLEncoding.EncodeToString([]byte(
				fmt.Sprintf("%s#%s", hook.HTTPSURL, hook.RefName),
			))

			args := []string{
				runOpts.Exec,
				jobID,
				hook.RefName,
				hook.RefType,
				hook.Owner,
				hook.Repo,
				hook.HTTPSURL,
			}
			cmd := exec.Command("bash", args...)

			env := os.Environ()
			envs := []string{
				"GIT_DEPLOY_JOB_ID=" + jobID,
				"GIT_REF_NAME=" + hook.RefName,
				"GIT_REF_TYPE=" + hook.RefType,
				"GIT_REPO_OWNER=" + hook.Owner,
				"GIT_REPO_NAME=" + hook.Repo,
				"GIT_CLONE_URL=" + hook.HTTPSURL,
			}
			cmd.Env = append(env, envs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if _, exists := jobs[jobID]; exists {
				// TODO put job in backlog
				log.Printf("git-deploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
				return
			}

			if err := cmd.Start(); nil != err {
				log.Printf("git-deploy exec error: %s\n", err)
				return
			}

			jobs[jobID] = &job{
				ID:  jobID,
				Cmd: cmd,
				Ref: hook,
			}

			go func() {
				log.Printf("git-deploy job for %s#%s started\n", hook.HTTPSURL, hook.RefName)
				_ = cmd.Wait()
				delete(jobs, jobID)
				log.Printf("git-deploy job for %s#%s finished\n", hook.HTTPSURL, hook.RefName)
			}()
		}
	}()

	if err := srv.ListenAndServe(); nil != err {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
		return
	}
}
