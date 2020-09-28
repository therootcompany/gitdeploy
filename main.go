package main

import (
	"compress/flate"
	"flag"
	"fmt"
	"net/http"
	"os"
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
	runFlags.StringVar(&runOpts.ServePath, "serve-path", "", "path to serve, falls back to built-in web app")
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
			fmt.Println(hook.Org)
			fmt.Println(hook.Repo)
			fmt.Println(hook.Branch)
		}
	}()

	if err := srv.ListenAndServe(); nil != err {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
		return
	}
}
