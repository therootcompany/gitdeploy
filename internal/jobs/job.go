package jobs

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

var initialized = false
var done = make(chan struct{})

// Start starts the job loop, channels, and cleanup routines
func Start(runOpts *options.ServerConfig) {
	go Run(runOpts)
}

// Run starts the job loop and waits for it to be stopped
func Run(runOpts *options.ServerConfig) {
	log.Printf("Starting")
	if initialized {
		panic(errors.New("should not double initialize 'jobs'"))
	}
	initialized = true

	// TODO load the backlog from disk

	ticker := time.NewTicker(runOpts.StaleAge / 2)
	for {
		select {
		case h := <-webhooks.Hooks:
			hook := webhooks.New(h)
			log.Printf("Saving to backlog and debouncing")
			saveBacklog(hook, runOpts)
			debounce(hook, runOpts)
		case hook := <-debacklog:
			log.Printf("Pulling from backlog and debouncing")
			debounce(hook, runOpts)
		case hook := <-debounced:
			log.Printf("Debounced by timer and running")
			run(hook, runOpts)
		case jobID := <-deathRow:
			// should !nokill (so... should kill job on the spot?)
			log.Printf("Removing after running exited, or being killed")
			remove(jobID /*, false*/)
		case promotion := <-Promotions:
			log.Printf("Promoting from %s to %s", promotion.GitRef.RefName, promotion.PromoteTo)
			promote(webhooks.New(*promotion.GitRef), promotion.PromoteTo, runOpts)
		case <-ticker.C:
			log.Printf("Running cleanup for expired, exited jobs")
			expire(runOpts)
		case <-done:
			log.Printf("Stopping")
			// TODO kill jobs
			ticker.Stop()
		}
	}
}

// Stop will cancel the job loop and its timers
func Stop() {
	done <- struct{}{}
	initialized = false
}

// Promotions channel
var Promotions = make(chan Promotion)

// Promotion is a channel message
type Promotion struct {
	PromoteTo string
	GitRef    *webhooks.Ref
}

// Jobs is the map of jobs
var Jobs = make(map[URLSafeRefID]*Job)

// Recents are jobs that are dead, but recent
var Recents = make(map[URLSafeRefID]*Job)

// deathRow is for jobs to be killed
var deathRow = make(chan URLSafeRefID)

// debounced is for jobs that are ready to run
var debounced = make(chan *webhooks.Ref)

// debacklog is for debouncing without saving in the backlog
var debacklog = make(chan *webhooks.Ref)

// URLSafeRefID is a newtype for the JobID
type URLSafeRefID string

// KillMsg describes which job to kill
type KillMsg struct {
	JobID string `json:"job_id"`
	Kill  bool   `json:"kill"`
}

// Job represents a job started by the git webhook
// and also the JSON we send back through the API about jobs
type Job struct {
	// normal json
	StartedAt time.Time     `json:"started_at"`
	JobID     string        `json:"job_id"`
	ExitCode  *int          `json:"exit_code"`
	GitRef    *webhooks.Ref `json:"ref"`
	Promote   bool          `json:"promote,omitempty"`
	EndedAt   time.Time     `json:"ended_at,omitempty"`
	// extra
	Logs   []Log      `json:"logs"`
	Report JobReport  `json:"report"`
	Cmd    *exec.Cmd  `json:"-"`
	mux    sync.Mutex `json:"-"`
}

// JobReport should have many items
type JobReport struct {
	Items []string
}

func getTimestamp(t time.Time) time.Time {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t
}

// All returns all jobs, including active, recent, and (TODO) historical
func All() []*Job {
	jobsCopy := []*Job{}

	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()
	for i := range Jobs {
		job := Jobs[i]
		jobCopy := &Job{
			StartedAt: job.StartedAt,
			JobID:     job.GitRef.GetRefID(),
			GitRef:    job.GitRef,
			Promote:   job.Promote,
			EndedAt:   job.EndedAt,
		}
		if nil != job.ExitCode {
			jobCopy.ExitCode = &(*job.ExitCode)
		}
		jobsCopy = append(jobsCopy, jobCopy)
	}
	// and here too
	for i := range Recents {
		job := Recents[i]
		jobCopy := &Job{
			StartedAt: job.StartedAt,
			JobID:     job.GitRef.GetRefID(),
			GitRef:    job.GitRef,
			Promote:   job.Promote,
			EndedAt:   job.EndedAt,
		}
		if nil != job.ExitCode {
			jobCopy.ExitCode = &(*job.ExitCode)
		}
		jobsCopy = append(jobsCopy, jobCopy)
	}

	return jobsCopy
}

// Remove will put a job on death row
func Remove(urlRefID URLSafeRefID /*, nokill bool*/) {
	deathRow <- urlRefID
}

func getEnvs(addr, jobID string, repoList string, hook *webhooks.Ref) []string {

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

func setOutput(logDir string, job *Job) *os.File {
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
func remove(jobID URLSafeRefID /*, nokill bool*/) {
	// TODO should have a mutex
	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()
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
	}
	if nil != job.Cmd.ProcessState {
		//*job.ExitCode = job.Cmd.ProcessState.ExitCode()
		exitCode := job.Cmd.ProcessState.ExitCode()
		job.ExitCode = &exitCode
	}
	job.EndedAt = time.Now()
}

func expire(runOpts *options.ServerConfig) {
	// TODO mutex here again (or switch to sync.Map)
	staleJobIDs := []URLSafeRefID{}
	jobsTimersMux.Lock()
	for jobID, job := range Recents {
		if time.Now().Sub(job.GitRef.Timestamp) > runOpts.StaleAge {
			staleJobIDs = append(staleJobIDs, jobID)
		}
	}
	// and here
	for i := range staleJobIDs {
		delete(Recents, staleJobIDs[i])
	}
	jobsTimersMux.Unlock()
}

// Log is a log message
type Log struct {
	Timestamp time.Time `json:"timestamp"`
	Stderr    bool
	Text      string
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
