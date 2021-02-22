package options

import (
	"flag"
	"time"
)

// Server is an instance of the config
var Server *ServerConfig

// ServerConfig is an options struct
type ServerConfig struct {
	Addr          string
	TrustProxy    bool
	RepoList      string
	Compress      bool
	ServePath     string
	ScriptsPath   string
	Promotions    []string
	LogDir        string // where the job logs should go
	TmpDir        string // where the backlog files go
	DebounceDelay time.Duration
	StaleAge      time.Duration // how old a dead job is before it's stale
	// TODO use BacklogDir instead?
}

// ServerFlags are the flags the web server can use
var ServerFlags *flag.FlagSet

// InitFlags are the flags for the main binary itself
var InitFlags *flag.FlagSet

// DefaultMaxBodySize is for the web server input
var DefaultMaxBodySize int64 = 1024 * 1024

// TimeFile is a time format like RFC3339, but filename-friendly
const TimeFile = "2006-01-02_15-04-05"

func init() {
	Server = &ServerConfig{}
	ServerFlags = flag.NewFlagSet("run", flag.ExitOnError)
	InitFlags = flag.NewFlagSet("init", flag.ExitOnError)
}
