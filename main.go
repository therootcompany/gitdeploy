package main

import (
	"compress/flate"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"git.ryanburnette.com/ryanburnette/git-deploy/assets"

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

type runOptions struct {
	listen     string
	trustProxy bool
	compress   bool
	static     string
}

var runFlags *flag.FlagSet
var runOpts runOptions
var initFlags *flag.FlagSet

var webhookProviders = make(map[string]func())
var webhooks = make(map[string]func(chi.Router))
var maxBodySize int64 = 1024 * 1024

var hooks chan webhook

type webhook struct {
	rev    string
	ref    string
	branch string
	repo   string
	org    string
}

func init() {
	hooks = make(chan webhook)
	runOpts = runOptions{}
	runFlags = flag.NewFlagSet("run", flag.ExitOnError)
	runFlags.StringVar(&runOpts.listen, "listen", ":3000", "the address and port on which to listen")
	runFlags.BoolVar(&runOpts.trustProxy, "trust-proxy", false, "trust X-Forwarded-For header")
	runFlags.BoolVar(&runOpts.compress, "compress", true, "enable compression for text,html,js,css,etc")
	runFlags.StringVar(&runOpts.static, "serve-path", "", "path to serve, falls back to built-in web app")
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
		initFlags.Parse(args[2:])
	case "run":
		runFlags.Parse(args[2:])
		registerWebhooks()
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
	if runOpts.trustProxy {
		r.Use(middleware.RealIP)
	}
	if runOpts.compress {
		r.Use(middleware.Compress(flate.DefaultCompression))
	}
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Recoverer)

	var staticHandler http.HandlerFunc
	pub := http.FileServer(assets.Assets)

	if len(runOpts.static) > 0 {
		// try the user-provided directory first, then fallback to the built-in
		devFS := http.Dir(runOpts.static)
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

	loadWebhooks(r)

	r.Get("/*", staticHandler)

	fmt.Println("Listening for http (with reasonable timeouts) on", runOpts.listen)
	srv := &http.Server{
		Addr:              runOpts.listen,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		MaxHeaderBytes:    1024 * 1024, // 1MiB
	}

	go func() {
		for {
			hook := <-hooks
			// TODO os.Exec
			fmt.Println(hook.org)
			fmt.Println(hook.repo)
			fmt.Println(hook.branch)
		}
	}()

	if err := srv.ListenAndServe(); nil != err {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
		return
	}
}

func registerWebhooks() {
	for _, add := range webhookProviders {
		add()
	}
}

func loadWebhooks(r chi.Router) {
	r.Route("/api/webhooks", func(r chi.Router) {
		for provider, handler := range webhooks {
			r.Route("/"+provider, func(r chi.Router) {
				handler(r)
			})
		}
	})
}
