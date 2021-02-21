package options

import (
	"flag"
)

var Server *ServerConfig

type ServerConfig struct {
	Addr        string
	TrustProxy  bool
	RepoList    string
	Compress    bool
	ServePath   string
	ScriptsPath string
	Promotions  []string
	// TODO LogDir
	// TODO BacklogDir
}

var ServerFlags *flag.FlagSet
var InitFlags *flag.FlagSet
var DefaultMaxBodySize int64 = 1024 * 1024

func init() {
	Server = &ServerConfig{}
	ServerFlags = flag.NewFlagSet("run", flag.ExitOnError)
	InitFlags = flag.NewFlagSet("init", flag.ExitOnError)
}
