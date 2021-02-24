package jobs

import (
	"encoding/base64"
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

// Promotions channel
var Promotions = make(chan Promotion)

// Pending is the map of backlog jobs
// map[webhooks.RefID]*webhooks.GitRef
var Pending sync.Map

// Actives is the map of jobs
// map[webhooks.RefID]*Job
var Actives sync.Map

// Recents are jobs that are dead, but recent
// map[webhooks.RevID]*Job
var Recents sync.Map

// deathRow is for jobs to be killed
var deathRow = make(chan webhooks.RefID)

// debounced is for jobs that are ready to run
var debounced = make(chan *webhooks.Ref)

// debacklog is for debouncing without saving in the backlog
var debacklog = make(chan *webhooks.Ref)

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

	// TODO load the backlog from disk too

	oldJobs, err := WalkLogs(runOpts)
	if nil != err {
		panic(err)
	}
	for i := range oldJobs {
		job := oldJobs[i]
		job.ID = string(job.GitRef.GetRevID())
		Recents.Store(job.GitRef.GetRevID(), job)
	}

	ticker := time.NewTicker(runOpts.StaleJobAge / 2)
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
		case activeID := <-deathRow:
			// should !nokill (so... should kill job on the spot?)
			log.Printf("Removing after running exited, or being killed")
			remove(activeID /*, false*/)
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

// Promotion is a channel message
type Promotion struct {
	PromoteTo string
	GitRef    *webhooks.Ref
}

// KillMsg describes which job to kill
type KillMsg struct {
	JobID string `json:"job_id"`
	Kill  bool   `json:"kill"`
}

// Job represents a job started by the git webhook
// and also the JSON we send back through the API about jobs
type Job struct {
	// normal json
	StartedAt time.Time     `json:"started_at,omitempty"` // empty when pending
	ID        string        `json:"id"`                   // could be URLSafeRefID or URLSafeRevID
	ExitCode  *int          `json:"exit_code"`            // empty when running
	GitRef    *webhooks.Ref `json:"ref"`                  // always present
	Promote   bool          `json:"promote,omitempty"`    // empty when deploy and test
	EndedAt   time.Time     `json:"ended_at,omitempty"`   // empty when running
	// extra
	Logs   []Log      `json:"logs"`             // exist when requested
	Report *Report    `json:"report,omitempty"` // empty unless given
	Cmd    *exec.Cmd  `json:"-"`
	mux    sync.Mutex `json:"-"`
}

// Report should have many items
type Report struct {
	Name    string   `json:"name"`
	Status  string   `json:"status,omitempty"`
	Message string   `json:"message,omitempty"`
	Detail  string   `json:"detail,omitempty"`
	Results []Report `json:"results,omitempty"`
}

// All returns all jobs, including active, recent, and (TODO) historical
func All() []*Job {
	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()

	jobsCopy := []*Job{}

	Pending.Range(func(key, value interface{}) bool {
		hook := value.(*webhooks.Ref)
		jobCopy := &Job{
			//StartedAt: job.StartedAt,
			ID:     string(hook.GetURLSafeRefID()),
			GitRef: hook,
			//Promote:   job.Promote,
			//EndedAt:   job.EndedAt,
		}
		jobsCopy = append(jobsCopy, jobCopy)
		return true
	})

	Actives.Range(func(key, value interface{}) bool {
		job := value.(*Job)
		jobCopy := &Job{
			StartedAt: job.StartedAt,
			ID:        string(job.GitRef.GetURLSafeRefID()),
			GitRef:    job.GitRef,
			Promote:   job.Promote,
			EndedAt:   job.EndedAt,
		}
		if nil != job.ExitCode {
			jobCopy.ExitCode = &(*job.ExitCode)
		}
		jobsCopy = append(jobsCopy, jobCopy)
		return true
	})

	Recents.Range(func(key, value interface{}) bool {
		job := value.(*Job)
		jobCopy := &Job{
			StartedAt: job.StartedAt,
			ID:        string(job.GitRef.GetURLSafeRevID()),
			GitRef:    job.GitRef,
			Promote:   job.Promote,
			EndedAt:   job.EndedAt,
		}
		if nil != job.ExitCode {
			jobCopy.ExitCode = &(*job.ExitCode)
		}
		jobsCopy = append(jobsCopy, jobCopy)
		return true
	})

	return jobsCopy
}

// SetReport will update jobs' logs
func SetReport(urlRefID webhooks.URLSafeRefID, report *Report) error {
	b, err := base64.RawURLEncoding.DecodeString(string(urlRefID))
	if nil != err {
		return err
	}
	refID := webhooks.RefID(b)

	value, ok := Actives.Load(refID)
	if !ok {
		return errors.New("active job not found by " + string(refID))
	}
	job := value.(*Job)

	job.Report = report
	Actives.Store(refID, job)

	return nil
}

// Remove will put a job on death row
func Remove(gitID webhooks.URLSafeRefID /*, nokill bool*/) {
	activeID, err :=
		base64.RawURLEncoding.DecodeString(string(gitID))
	if nil != err {
		log.Printf("bad id: %s", activeID)
		return
	}
	deathRow <- webhooks.RefID(activeID)
}

func getEnvs(addr, activeID string, repoList string, hook *webhooks.Ref) []string {

	port := strings.Split(addr, ":")[1]

	envs := []string{
		"GIT_DEPLOY_JOB_ID=" + activeID,
		"GIT_DEPLOY_TIMESTAMP=" + hook.Timestamp.Format(time.RFC3339),
		"GIT_DEPLOY_CALLBACK_URL=" + "http://localhost:" + port + "/api/local/jobs/" + string(hook.GetURLSafeRefID()),
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
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	//return fmt.Sprintf("%s#%s", strings.ReplaceAll(hook.RepoID, "/", "-"), hook.RefName)
}

func openJobFile(baseDir string, hook *webhooks.Ref, suffix string) (*os.File, error) {
	repoDir, repoFile, _ := getJobFilePath(baseDir, hook, suffix)
	return os.Open(filepath.Join(repoDir, repoFile))
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

	cwd, _ := os.Getwd()
	log.Printf("["+hook.RepoID+"#"+hook.RefName+"] logging to '.%s'", f.Name()[len(cwd):])
	return f
}

// Remove kills the job and moves it to recents
func remove(activeID webhooks.RefID /*, nokill bool*/) {
	// Encapsulate the whole transaction
	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()

	value, ok := Actives.Load(activeID)
	if !ok {
		log.Printf("[warn] could not find job to kill by RefID %s", activeID)
		return
	}
	job := value.(*Job)
	Actives.Delete(activeID)

	// transition to RevID for non-active, non-pending jobs
	job.ID = string(job.GitRef.GetRevID())
	Recents.Store(job.GitRef.GetRevID(), job)

	updateExitStatus(job)

	// JSON should have been written to disk by this point
	job.Logs = []Log{}
}

func updateExitStatus(job *Job) {
	if nil == job.Cmd.ProcessState {
		// is not yet finished
		if nil != job.Cmd.Process {
			// but definitely was started
			if err := job.Cmd.Process.Kill(); nil != err {
				log.Printf("error killing job:\n%v", err)
			}
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
	staleJobIDs := []webhooks.URLSafeRevID{}

	Recents.Range(func(key, value interface{}) bool {
		revID := key.(webhooks.URLSafeRevID)
		age := time.Now().Sub(value.(*Job).GitRef.Timestamp)
		if age > runOpts.StaleJobAge {
			staleJobIDs = append(staleJobIDs, revID)
		}
		return true
	})

	for _, revID := range staleJobIDs {
		Recents.Delete(revID)
	}
}
