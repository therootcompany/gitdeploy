package jobs

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

// WalkLogs creates partial webhooks.Refs from walking the log dir
func WalkLogs(runOpts *options.ServerConfig) ([]*Job, error) {
	oldJobs := []*Job{}
	if 0 == len(runOpts.LogDir) {
		return oldJobs, nil
	}

	now := time.Now()
	pathLen := len(runOpts.LogDir + "/")
	err := filepath.WalkDir(runOpts.LogDir, func(logpath string, d fs.DirEntry, err error) error {
		if nil != err {
			log.Printf("failed to walk log dir: %v", err)
			return nil
		}
		if !d.Type().IsRegular() || '.' == logpath[0] || '_' == logpath[0] || '~' == logpath[0] {
			return nil
		}

		rel := logpath[pathLen:]
		paths := strings.Split(rel, "/")
		repoID := strings.Join(paths[:len(paths)-1], "/")
		repoName := paths[len(paths)-2]
		var repoOwner string
		//repoHost := paths[0]
		if len(paths) >= 4 {
			repoOwner = paths[len(paths)-3]
		}
		logname := paths[len(paths)-1]

		rev := strings.Split(logname, ".")
		if 4 != len(rev) {
			return nil
		}

		ts, _ := time.ParseInLocation(options.TimeFile, rev[0], time.UTC)
		age := now.Sub(ts)
		if age <= runOpts.StaleLogAge {
			if "json" == rev[3] {
				if f, err := os.Open(logpath); nil != err {
					log.Printf("[warn] failed to read log dir")
				} else {
					dec := json.NewDecoder(f)
					j := &Job{}
					if err := dec.Decode(j); nil == err {
						// don't keep all the logs in memory
						j.Logs = []Log{}
						j.ID = string(j.GitRef.GetRevID())
						oldJobs = append(oldJobs, j)
					}
				}
			} else {
				hook := &webhooks.Ref{
					HTTPSURL:  "//" + repoID + ".git",
					RepoID:    repoID,
					Owner:     repoOwner,
					Repo:      repoName,
					Timestamp: ts,
					RefName:   rev[1],
					Rev:       rev[2],
				}
				oldJobs = append(oldJobs, &Job{
					ID:     string(hook.GetRevID()),
					GitRef: hook,
				})
			}
		}

		// ExpiredLogAge can be 0 for testing,
		// even when StaleLogAge is > 0
		if age >= runOpts.ExpiredLogAge {
			log.Printf("[gitdeploy] remove %s", logpath)
			os.Remove(logpath)
		}

		return nil
	})

	return oldJobs, err
}

//func GetReport(runOpts *options.ServerConfig, safeID webhooks.URLSafeGitID) (*Job, error) {}

// LoadLogs will log logs for a job
func LoadLogs(runOpts *options.ServerConfig, safeID webhooks.URLSafeGitID) (*Job, error) {
	b, err := base64.RawURLEncoding.DecodeString(string(safeID))
	if nil != err {
		return nil, err
	}

	gitID := string(b)
	refID := webhooks.RefID(gitID)
	revID := webhooks.RevID(gitID)

	var f *os.File = nil
	if value, ok := Actives.Load(refID); ok {
		j := value.(*Job)
		f, err = openJobFile(runOpts.LogDir, j.GitRef, ".json")
		if nil != err {
			return nil, err
		}
	} else if value, ok := Recents.Load(revID); ok {
		j := value.(*Job)
		f, err = openJobFile(runOpts.LogDir, j.GitRef, ".json")
		if nil != err {
			return nil, err
		}
	}

	if nil == f {
		return nil, errors.New("no job found")
	}
	dec := json.NewDecoder(f)
	j := &Job{}
	if err := dec.Decode(j); nil != err {
		return nil, errors.New("couldn't read log file")
	}
	j.ID = string(gitID)

	return j, nil
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
