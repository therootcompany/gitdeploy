package jobs

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
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

// Pending is the map of backlog jobs
// map[webhooks.RefID]*webhooks.GitRef
var Pending sync.Map

// Actives is the map of jobs
// map[webhooks.RefID]*Job
var Actives sync.Map

// Recents are jobs that are dead, but recent
// map[webhooks.RevID]*Job
var Recents sync.Map

// Job represents a job started by the git webhook
// and also the JSON we send back through the API about jobs
type Job struct {
	// normal json
	StartedAt time.Time     `json:"started_at,omitempty"` // empty when pending
	ID        string        `json:"id"`                   // could be URLSafeRefID or URLSafeRevID
	GitRef    *webhooks.Ref `json:"ref"`                  // always present
	PromoteTo string        `json:"promote_to,omitempty"` // empty when deploy and test
	Promote   bool          `json:"promote,omitempty"`    // empty when deploy and test
	EndedAt   time.Time     `json:"ended_at,omitempty"`   // empty when running
	ExitCode  *int          `json:"exit_code"`            // empty when running
	// full json
	Logs   []Log   `json:"logs"`             // exist when requested
	Report *Report `json:"report,omitempty"` // empty unless given
	// internal only
	cmd *exec.Cmd  `json:"-"`
	mux sync.Mutex `json:"-"`
}

// Report should have many items
type Report struct {
	Name    string   `json:"name"`
	Status  string   `json:"status,omitempty"`
	Message string   `json:"message,omitempty"`
	Detail  string   `json:"detail,omitempty"`
	Results []Report `json:"results,omitempty"`
}

var initialized = false
var done = make(chan struct{})
var deathRow = make(chan webhooks.RefID)
var debounced = make(chan *webhooks.Ref)
var debacklog = make(chan *webhooks.Ref)

// Start starts the job loop, channels, and cleanup routines
func Start(runOpts *options.ServerConfig) {
	go Run(runOpts)
}

// Run starts the job loop and waits for it to be stopped
func Run(runOpts *options.ServerConfig) {
	log.Printf("[gitdeploy] Starting")
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
			//log.Printf("[%s] debouncing...", hook.GetRefID())
			saveBacklog(hook, runOpts)
			debounce(hook, runOpts)
		case hook := <-debacklog:
			//log.Printf("[%s] checking for backlog...", hook.GetRefID())
			debounce(hook, runOpts)
		case hook := <-debounced:
			//log.Printf("[%s] debounced!", hook.GetRefID())
			run(hook, runOpts)
		case activeID := <-deathRow:
			//log.Printf("[%s] done", activeID)
			remove(activeID /*, false*/)
		case promotion := <-Promotions:
			log.Printf("[%s] promoting to %s", promotion.GitRef.GetRefID(), promotion.PromoteTo)
			promote(webhooks.New(*promotion.GitRef), promotion.PromoteTo, runOpts)
		case <-ticker.C:
			log.Printf("[gitdeploy] cleaning old jobs")
			expire(runOpts)
		case <-done:
			log.Printf("[gitdeploy] stopping")
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
			//Promote:   job.Promote,
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
			EndedAt:   job.EndedAt,
			//Promote:   job.Promote,
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
		cmd := job.cmd
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
	log.Printf("[%s] log to ./%s", hook.GetRefID(), f.Name()[len(cwd)+1:])
	return f
}

// Debounce puts a job in the queue, in time
func Debounce(hook webhooks.Ref) {
	webhooks.Hooks <- hook
}

var jobsTimersMux sync.Mutex
var debounceTimers = make(map[webhooks.RefID]*time.Timer)

func debounce(hook *webhooks.Ref, runOpts *options.ServerConfig) {
	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()

	activeID := hook.GetRefID()
	if _, ok := Actives.Load(activeID); ok {
		//log.Printf("[%s] will run again after current job", hook.GetRefID())
		return
	}

	refID := hook.GetRefID()
	timer, ok := debounceTimers[refID]
	if ok {
		//log.Printf("[%s] replaced debounce timer", hook.GetRefID())
		timer.Stop()
	}
	// this will not cause a mutual lock because it is async
	debounceTimers[refID] = time.AfterFunc(runOpts.DebounceDelay, func() {
		jobsTimersMux.Lock()
		delete(debounceTimers, refID)
		jobsTimersMux.Unlock()

		debounced <- hook
	})
}

func saveBacklog(hook *webhooks.Ref, runOpts *options.ServerConfig) {
	pendingID := hook.GetRefID()
	Pending.Store(pendingID, hook)

	repoDir, repoFile, err := getBacklogFilePath(runOpts.TmpDir, hook)
	if nil != err {
		log.Printf("[warn] could not create backlog dir %s:\n%v", repoDir, err)
		return
	}
	f, err := ioutil.TempFile(repoDir, "tmp-*")
	if nil != err {
		log.Printf("[warn] could not create backlog file %s:\n%v", f.Name(), err)
		return
	}

	b, _ := json.MarshalIndent(hook, "", "  ")
	if _, err := f.Write(b); nil != err {
		log.Printf("[warn] could not write backlog file %s:\n%v", f.Name(), err)
		return
	}

	replace := false
	backlogPath := filepath.Join(repoDir, repoFile)
	if _, err := os.Stat(backlogPath); nil == err {
		replace = true
		_ = os.Remove(backlogPath)
	}
	if err := os.Rename(f.Name(), backlogPath); nil != err {
		log.Printf("[warn] rename backlog json failed:\n%v", err)
		return
	}

	if replace {
		log.Printf("[%s] updated in queue", hook.GetRefID())
	} else {
		log.Printf("[%s] added to queue", hook.GetRefID())
	}
}

func getBacklogFilePath(baseDir string, hook *webhooks.Ref) (string, string, error) {
	baseDir, _ = filepath.Abs(baseDir)
	fileName := hook.RefName + ".json"
	fileDir := filepath.Join(baseDir, hook.RepoID)

	err := os.MkdirAll(fileDir, 0755)

	return fileDir, fileName, err
}

func run(curHook *webhooks.Ref, runOpts *options.ServerConfig) {
	// because we want to lock the whole transaction all of the state
	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()

	pendingID := curHook.GetRefID()
	if _, ok := Actives.Load(pendingID); ok {
		log.Printf("[%s] already in progress", pendingID)
		return
	}

	var hook *webhooks.Ref
	// Legacy, but would be nice to repurpose for resuming on reload
	repoDir, repoFile, _ := getBacklogFilePath(runOpts.TmpDir, curHook)
	backlogFile := filepath.Join(repoDir, repoFile)
	if value, ok := Pending.Load(pendingID); ok {
		hook = value.(*webhooks.Ref)
	} else {
		// TODO add mutex (should not affect temp files)
		_ = os.Remove(backlogFile + ".cur")
		_ = os.Rename(backlogFile, backlogFile+".cur")
		b, err := ioutil.ReadFile(backlogFile + ".cur")
		if nil != err {
			if !os.IsNotExist(err) {
				log.Printf("[warn] could not read backlog file %s:\n%v", backlogFile, err)
				return
			}
			// doesn't exist => no backlog
			log.Printf("[%s] no backlog", pendingID)
			return
		}

		hook = &webhooks.Ref{}
		if err := json.Unmarshal(b, hook); nil != err {
			log.Printf("[warn] could not parse backlog %s:\n%v", backlogFile, err)
			return
		}
		hook = webhooks.New(*hook)
	}

	Pending.Delete(pendingID)
	_ = os.Remove(backlogFile)
	_ = os.Remove(backlogFile + ".cur")

	env := os.Environ()
	envs := getEnvs(runOpts.Addr, string(pendingID), runOpts.RepoList, hook)
	envs = append(envs, "GIT_DEPLOY_JOB_ID="+string(pendingID))

	scriptPath, _ := filepath.Abs(runOpts.ScriptsPath + "/deploy.sh")
	args := []string{"-i", "--", scriptPath}

	log.Printf("[%s] bash %s %s %s", hook.GetRefID(), args[0], args[1], args[2])
	cmd := exec.Command("bash", append(args, []string{
		string(pendingID),
		hook.RefName,
		hook.RefType,
		hook.Owner,
		hook.Repo,
		hook.HTTPSURL,
	}...)...)
	cmd.Env = append(env, envs...)

	now := time.Now()
	j := &Job{
		StartedAt: now,
		cmd:       cmd,
		GitRef:    hook,
		Logs:      []Log{},
		Promote:   false,
	}
	// TODO jobs.New()
	// Sets cmd.Stdout and cmd.Stderr
	txtFile := setOutput(runOpts.LogDir, j)

	if err := cmd.Start(); nil != err {
		log.Printf("[ERROR] failed to exec: %s\n", err)
		return
	}

	Actives.Store(pendingID, j)

	go func() {
		//log.Printf("[%s] job started", pendingID)
		if err := cmd.Wait(); nil != err {
			log.Printf("[%s] exited with error: %v", pendingID, err)
		} else {
			log.Printf("[%s] exited successfully", pendingID)
		}
		if nil != txtFile {
			_ = txtFile.Close()
		}

		// TODO move to deathRow only?
		updateExitStatus(j)

		// Switch ID to the more specific RevID
		j.ID = string(j.GitRef.GetRevID())
		// replace the text log with a json log
		if jsonFile, err := getJobFile(runOpts.LogDir, j.GitRef, ".json"); nil != err {
			// jsonFile.Name() should be the full path
			log.Printf("[warn] could not create log file '%s': %v", runOpts.LogDir, err)
		} else {
			enc := json.NewEncoder(jsonFile)
			enc.SetIndent("", "  ")
			if err := enc.Encode(j); nil != err {
				log.Printf("[warn] could not encode json log '%s': %v", jsonFile.Name(), err)
			} else {
				logdir, logname, _ := getJobFilePath(runOpts.LogDir, j.GitRef, ".log")
				_ = os.Remove(filepath.Join(logdir, logname))
			}
			_ = jsonFile.Close()
		}

		// TODO move to deathRow only?
		j.Logs = []Log{}

		// this will completely clear the finished job
		deathRow <- pendingID

		// debounces without saving in the backlog
		// TODO move this into deathRow?
		debacklog <- hook
	}()
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
	if nil == job.cmd.ProcessState {
		// is not yet finished
		if nil != job.cmd.Process {
			// but definitely was started
			if err := job.cmd.Process.Kill(); nil != err {
				log.Printf("error killing job:\n%v", err)
			}
		}
	}
	if nil != job.cmd.ProcessState {
		//*job.ExitCode = job.cmd.ProcessState.ExitCode()
		exitCode := job.cmd.ProcessState.ExitCode()
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
