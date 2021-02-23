package main

import (
	"compress/flate"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.rootprojects.org/root/gitdeploy/assets/examples"
	"git.rootprojects.org/root/gitdeploy/assets/public"
	"git.rootprojects.org/root/gitdeploy/internal/api"
	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
	"git.rootprojects.org/root/vfscopy"

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
	fmt.Println(ver())
	fmt.Println("")
	fmt.Printf("Use '%s help <command>'\n", name)
	fmt.Println("  init")
	fmt.Println("  run")
}

func ver() string {
	return fmt.Sprintf("%s v%s (%s) %s", name, version, commit[:7], date)
}

var runOpts *options.ServerConfig
var runFlags *flag.FlagSet
var initFlags *flag.FlagSet
var promotions []string
var promotionList string
var defaultPromotionList = "production,staging,master"
var oldScripts string

func init() {
	runOpts = options.Server

	initFlags = options.InitFlags
	_ = initFlags.Bool("TODO", false, "init will eventually copy default assets into a local directory")

	runFlags = options.ServerFlags
	runFlags.StringVar(&runOpts.Addr, "listen", "", "the address and port on which to listen (default :4483)")
	runFlags.BoolVar(&runOpts.TrustProxy, "trust-proxy", false, "trust X-Forwarded-For header")
	runFlags.StringVar(&runOpts.RepoList, "trust-repos", "",
		"list of repos (ex: 'github.com/org/repo', or '*' for all) for which to run '.gitdeploy/deploy.sh'")
	runFlags.BoolVar(&runOpts.Compress, "compress", true, "enable compression for text,html,js,css,etc")
	runFlags.StringVar(
		&runOpts.ServePath, "serve-path", "",
		"path to serve, falls back to built-in web app")
	runFlags.StringVar(
		&oldScripts, "exec", "",
		"old alias for --scripts")
	runFlags.StringVar(
		&runOpts.ScriptsPath, "scripts", "",
		"path to ./scripts/{deploy.sh,promote.sh,etc}")
	//"path to bash script to run with git info as arguments")
	runFlags.StringVar(&promotionList, "promotions", "",
		"a list of promotable branches in descending order (default '"+defaultPromotionList+"')")
}

func main() {
	// Support [--]version and -V
	if len(os.Args) > 1 {
		if "version" == strings.TrimLeft(os.Args[1], "-") || "-V" == os.Args[1] {
			fmt.Println(ver())
			os.Exit(0)
			return
		}
	}

	args := os.Args[:]
	if 1 == len(args) {
		// "run" should be the default
		args = append(args, "run")
	}

	if "help" == strings.TrimLeft(args[1], "-") {
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
		fmt.Println(ver())
		os.Exit(0)
		return
	case "init":
		_ = initFlags.Parse(args[2:])
		gdInit()
		os.Exit(0)
		return
	case "run":
		_ = runFlags.Parse(args[2:])
		if "" == runOpts.ScriptsPath {
			if "" != oldScripts {
				fmt.Fprintf(os.Stderr, "--exec is deprecated and will be removed. Please use --scripts instead.\n")
				runOpts.ScriptsPath = oldScripts
			}
		}
		if "" == runOpts.ScriptsPath {
			runOpts.ScriptsPath = "./scripts"
			pathname, _ := filepath.Abs("./scripts")
			if info, _ := os.Stat("./scripts/deploy.sh"); nil == info || !info.Mode().IsRegular() {
				fmt.Printf(
					"%q not found.\nPlease provide --scripts ./scripts/ as a path where \"deploy.sh\" can be found\n",
					pathname,
				)
				os.Exit(1)
			}
		}
		if 0 == len(runOpts.Addr) {
			runOpts.Addr = os.Getenv("LISTEN")
		}
		if 0 == len(runOpts.Addr) {
			port := os.Getenv("PORT")
			if len(port) > 0 {
				runOpts.Addr = "localhost:" + port
			}
		}
		if 0 == len(runOpts.Addr) {
			runOpts.Addr = "localhost:4483"
		}
		if 0 == len(runOpts.Addr) {
			fmt.Printf("--listen <[addr]:port> is a required flag")
			os.Exit(1)
			return
		}
		if 0 == len(runOpts.RepoList) {
			runOpts.RepoList = os.Getenv("TRUST_REPOS")
		}
		if 0 == len(runOpts.LogDir) {
			runOpts.LogDir = os.Getenv("LOG_DIR")
		}
		if 0 == len(runOpts.TmpDir) {
			var err error
			runOpts.TmpDir, err = ioutil.TempDir("", "gitdeploy-*")
			if nil != err {
				fmt.Fprintf(os.Stderr, "could not create temporary directory")
				os.Exit(1)
				return
			}
			log.Printf("TEMP_DIR=%s", runOpts.TmpDir)
		}
		if 0 == runOpts.DebounceDelay {
			runOpts.DebounceDelay = 2 * time.Second
		}
		if 0 == runOpts.StaleJobAge {
			runOpts.StaleJobAge = 30 * time.Minute
		}
		if 0 == runOpts.StaleLogAge {
			runOpts.StaleLogAge = 15 * 24 * time.Hour
		}
		if 0 == runOpts.ExpiredLogAge {
			runOpts.ExpiredLogAge = 90 * 24 * time.Hour
		}

		if len(runOpts.RepoList) > 0 {
			runOpts.RepoList = strings.ReplaceAll(runOpts.RepoList, ",", " ")
			runOpts.RepoList = strings.ReplaceAll(runOpts.RepoList, "  ", " ")
			runOpts.RepoList = strings.ToLower(runOpts.RepoList)
		}
		cwd, _ := os.Getwd()
		log.Printf("WORK_DIR=%s", cwd)
		log.Printf("TRUST_REPOS=%s", runOpts.RepoList)
		if 0 == len(promotionList) {
			promotionList = os.Getenv("PROMOTIONS")
		}
		if 0 == len(promotionList) {
			promotionList = defaultPromotionList
		}
		runOpts.Promotions = strings.Fields(
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

func gdInit() {
	vfs := vfscopy.NewVFS(examples.Assets)
	_, err := os.Open("scripts")
	fmt.Println("Initiazing ...")
	if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "    skip: ./scripts already exists\n")
	} else {
		if err := vfscopy.CopyAll(vfs, ".", "./scripts", vfscopy.Options{
			AddPermission: os.FileMode(0600),
			Skip: func(path string) (bool, error) {
				if strings.HasSuffix(path, "/dotenv") {
					return true, nil
				}

				f, _ := vfs.Open(path)
				fi, _ := f.Stat()
				if !fi.IsDir() {
					fmt.Println("    copy: scripts/" + path)
				}
				return false, nil
			},
		}); nil != err {
			fmt.Fprintf(os.Stderr, "error initializing ./scripts directory: %v\n", err)
			os.Exit(1)
			return
		}
	}
	_, err = os.Open(".env")
	if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "    skip: ./.env already exists\n")
	} else {
		if err := vfscopy.CopyAll(vfs, "dotenv", ".env", vfscopy.Options{
			AddPermission: os.FileMode(0600),
		}); nil != err {
			fmt.Fprintf(os.Stderr, "error initializing ./.env file: %v\n", err)
			os.Exit(1)
			return
		}
		_ = os.Chmod(".env", 0600)
		fmt.Println("    copy: .env")
	}
	fmt.Println("Done.")
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

	r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Write(append([]byte(ver()), '\n'))
	})
	r.Get("/api/version", func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.MarshalIndent(struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Date    string `json:"date"`
			Commit  string `json:"commit"`
		}{
			Name:    name,
			Version: version,
			Date:    date,
			Commit:  commit,
		}, "", "  ")
		w.Write(append(b, '\n'))
	})
	api.Route(r, runOpts)

	var staticHandler http.HandlerFunc
	pub := http.FileServer(public.Assets)

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
	if err := srv.ListenAndServe(); nil != err {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
		return
	}
}
