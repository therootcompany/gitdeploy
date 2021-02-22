package jobs

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

// WalkLogs creates partial webhooks.Refs from walking the log dir
func WalkLogs(runOpts *options.ServerConfig) ([]webhooks.Ref, error) {
	hooks := []webhooks.Ref{}
	if 0 == len(runOpts.LogDir) {
		return hooks, nil
	}

	pathLen := len(runOpts.LogDir + "/")
	err := filepath.WalkDir(runOpts.LogDir, func(path string, d fs.DirEntry, err error) error {
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

		ts, _ := time.ParseInLocation(options.TimeFile, rev[0], time.UTC)
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

// Log is a log message
type Log struct {
	Timestamp time.Time `json:"timestamp"`
	Stderr    bool      `json:"stderr"`
	Text      string    `json:"text"`
}

type outWriter struct {
	//io.Writer
	job *Job
}

func (w outWriter) Write(b []byte) (int, error) {
	w.job.mux.Lock()
	w.job.Logs = append(w.job.Logs, Log{
		Timestamp: time.Now().UTC(),
		Stderr:    false,
		Text:      string(b),
	})
	w.job.mux.Unlock()
	return len(b), nil
}

type errWriter struct {
	//io.Writer
	job *Job
}

func (w errWriter) Write(b []byte) (int, error) {
	w.job.mux.Lock()
	w.job.Logs = append(w.job.Logs, Log{
		Timestamp: time.Now().UTC(),
		Stderr:    true,
		Text:      string(b),
	})
	w.job.mux.Unlock()
	return len(b), nil
}
