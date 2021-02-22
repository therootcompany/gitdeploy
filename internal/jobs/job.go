package jobs

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

var initialized = false

// Init starts the channel and cleanup routines
func Init(runOpts *options.ServerConfig) {
	if initialized {
		panic(errors.New("should not double initialize 'jobs'"))
	}
	initialized = true

	go func() {
		// TODO read from backlog
		for {
			//hook := webhooks.Accept()
			select {
			case promotion := <-Promotions:
				promote(webhooks.New(*promotion.Ref), promotion.PromoteTo, runOpts)
			case hook := <-webhooks.Hooks:
				debounce(webhooks.New(hook), runOpts)
			case jobID := <-deathRow:
				// should !nokill (so... should kill job on the spot?)
				remove(jobID /*, false*/)
			}
		}
	}()

	go func() {
		for {
			time.Sleep(time.Minute)

			// TODO mutex here again (or switch to sync.Map)
			staleJobIDs := []string{}
			for jobID, job := range Recents {
				if time.Now().Sub(job.CreatedAt) > time.Hour {
					staleJobIDs = append(staleJobIDs, jobID)
				}
			}
			// and here
			for i := range staleJobIDs {
				delete(Recents, staleJobIDs[i])
			}
		}
	}()
}

// Promotions channel
var Promotions = make(chan Promotion)

// Promotion is a channel message
type Promotion struct {
	PromoteTo string
	Ref       *webhooks.Ref
}

// Jobs is the map of jobs
var Jobs = make(map[string]*HookJob)

// Recents are jobs that are dead, but recent
var Recents = make(map[string]*HookJob)

// deathRow is for jobs to be killed
var deathRow = make(chan JobID)

// JobID is a type alias
type JobID = string

// KillMsg describes which job to kill
type KillMsg struct {
	JobID string `json:"job_id"`
	Kill  bool   `json:"kill"`
}

// Job is the JSON we send back through the API about jobs
type Job struct {
	JobID     string        `json:"job_id"`
	CreatedAt time.Time     `json:"created_at"`
	GitRef    *webhooks.Ref `json:"ref"`
	Promote   bool          `json:"promote,omitempty"`
}

// All returns all jobs, including active, recent, and (TODO) historical
func All() []Job {
	// again, possible race condition, but not one that much matters
	jobsCopy := []Job{}
	for jobID, job := range Jobs {
		jobsCopy = append(jobsCopy, Job{
			JobID:     jobID,
			GitRef:    job.GitRef,
			CreatedAt: job.CreatedAt,
		})
	}
	// and here too
	for jobID, job := range Recents {
		jobsCopy = append(jobsCopy, Job{
			JobID:     jobID,
			GitRef:    job.GitRef,
			CreatedAt: job.CreatedAt,
		})
	}

	return jobsCopy
}

// Remove will put a job on death row
func Remove(jobID JobID /*, nokill bool*/) {
	deathRow <- jobID
}

func getEnvs(addr, jobID string, repoList string, hook *webhooks.Ref) []string {
	hook.RepoID = getRepoID(hook.HTTPSURL)
	hook.Timestamp = getTimestamp(hook.Timestamp)

	port := strings.Split(addr, ":")[1]

	envs := []string{
		"GIT_DEPLOY_JOB_ID=" + jobID,
		"GIT_DEPLOY_TIMESTAMP=" + hook.Timestamp.Format(time.RFC3339),
		"GIT_DEPLOY_CALLBACK_URL=" + "http://localhost:" + port + "/api/jobs/" + jobID,
		"GIT_REF_NAME=" + hook.RefName,
		"GIT_REF_TYPE=" + hook.RefType,
		"GIT_REPO_ID=" + hook.RepoID,
		"GIT_REPO_OWNER=" + hook.Owner,
		"GIT_REPO_NAME=" + hook.Repo,
		"GIT_CLONE_URL=" + hook.HTTPSURL, // deprecated
		"GIT_HTTPS_URL=" + hook.HTTPSURL,
		"GIT_SSH_URL=" + hook.SSHURL,
	}

	// GIT_REPO_TRUSTED
	// Set GIT_REPO_TRUSTED=TRUE if the repo matches exactly, or by pattern
	repoID := strings.ToLower(hook.RepoID)
	for _, repo := range strings.Fields(repoList) {
		last := len(repo) - 1
		if len(repo) < 0 {
			continue
		}
		repo = strings.ToLower(repo)
		if '*' == repo[last] {
			// Wildcard match a prefix, for example:
			// github.com/whatever/*					MATCHES github.com/whatever/foo
			// github.com/whatever/ProjectX-* MATCHES github.com/whatever/ProjectX-Foo
			if strings.HasPrefix(repoID, repo[:last]) {
				envs = append(envs, "GIT_REPO_TRUSTED=true")
				break
			}
		} else if repo == repoID {
			envs = append(envs, "GIT_REPO_TRUSTED=true")
			break
		}
	}

	return envs
}

func getJobFilePath(baseDir string, hook *webhooks.Ref, suffix string) (string, string, error) {
	baseDir, _ = filepath.Abs(baseDir)
	fileTime := hook.Timestamp.UTC().Format(options.TimeFile)
	fileName := fileTime + "." + hook.RefName + "." + hook.Rev[:7] + suffix // ".log" or ".json"
	fileDir := filepath.Join(baseDir, hook.RepoID)

	err := os.MkdirAll(fileDir, 0755)

	return fileDir, fileName, err
}

func getJobFile(baseDir string, hook *webhooks.Ref, suffix string) (*os.File, error) {
	repoDir, repoFile, err := getJobFilePath(baseDir, hook, suffix)
	if nil != err {
		//log.Printf("[warn] could not create log directory '%s': %v", repoDir, err)
		return nil, err
	}

	path := filepath.Join(repoDir, repoFile)
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0755)
	//return fmt.Sprintf("%s#%s", strings.ReplaceAll(hook.RepoID, "/", "-"), hook.RefName)
}

func getJobID(hook *webhooks.Ref) string {
	return base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.RepoID, hook.RefName),
	))
}

func setOutput(logDir string, job *HookJob) *os.File {
	var f *os.File = nil

	defer func() {
		// TODO write to append-only log rather than keep in-memory
		// (noting that we want to keep Stdout vs Stderr and timing)
		cmd := job.Cmd
		wout := &outWriter{job: job}
		werr := &outWriter{job: job}
		if nil != f {
			cmd.Stdout = io.MultiWriter(f, wout)
			cmd.Stderr = io.MultiWriter(f, werr)
		} else {
			cmd.Stdout = io.MultiWriter(os.Stdout, wout)
			cmd.Stderr = io.MultiWriter(os.Stderr, werr)
		}
	}()

	if "" == logDir {
		return nil
	}

	hook := job.GitRef
	f, err := getJobFile(logDir, hook, ".log")
	if nil != err {
		// f.Name() should be the full path
		log.Printf("[warn] could not create log file '%s': %v", logDir, err)
		return nil
	}

	log.Printf("["+hook.RepoID+"#"+hook.RefName+"] logging to '%s'", f.Name())
	return f
}

// Remove kills the job and moves it to recents
func remove(jobID JobID /*, nokill bool*/) {
	// TODO should have a mutex
	job, exists := Jobs[jobID]
	if !exists {
		return
	}
	delete(Jobs, jobID)
	// TODO write log as JSON
	Recents[jobID] = job

	if nil == job.Cmd.ProcessState {
		// is not yet finished
		if nil != job.Cmd.Process {
			// but definitely was started
			err := job.Cmd.Process.Kill()
			log.Printf("error killing job:\n%v", err)
		}
	} else {
		//*job.ExitCode = job.Cmd.ProcessState.ExitCode()
		exitCode := job.Cmd.ProcessState.ExitCode()
		job.ExitCode = &exitCode
	}
}

// Log is a log message
type Log struct {
	Timestamp time.Time `json:"timestamp"`
	Stderr    bool
	Text      string
}

type outWriter struct {
	//io.Writer
	job *HookJob
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
	job *HookJob
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
