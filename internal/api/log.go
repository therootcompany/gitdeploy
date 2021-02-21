package api

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

// TimeFile is a time format like RFC3339, but filename-friendly
const TimeFile = "2006-01-02_15-04-05"

// WalkLogs creates partial webhooks.Refs from walking the log dir
func WalkLogs(logDir string) ([]webhooks.Ref, error) {
	hooks := []webhooks.Ref{}
	if 0 == len(logDir) {
		return hooks, nil
	}

	pathLen := len(logDir + "/")
	err := filepath.WalkDir(logDir, func(path string, d fs.DirEntry, err error) error {
		if !d.Type().IsRegular() || '.' == path[0] || '_' == path[0] || '~' == path[0] {
			return nil
		}

		rel := path[pathLen:]
		paths := strings.Split(rel, "/")
		repoID := strings.Join(paths[:len(paths)-1], "/")
		repoName := paths[len(paths)-2]
		var repoOwner string
		//repoHost := paths[0]
		if len(paths) >= 4 {
			repoOwner = paths[len(paths)-3]
		}
		log := paths[len(paths)-1]

		rev := strings.Split(log, ".")
		if 4 != len(rev) {
			return nil
		}

		ts, _ := time.ParseInLocation(TimeFile, rev[0], time.UTC)
		hooks = append(hooks, webhooks.Ref{
			HTTPSURL:  "//" + repoID + ".git",
			RepoID:    repoID,
			Owner:     repoOwner,
			Repo:      repoName,
			Timestamp: ts,
			RefName:   rev[1],
			Rev:       rev[2],
		})

		return nil
	})

	return hooks, err
}
