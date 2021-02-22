package jobs

import (
	"encoding/json"
	"fmt"
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

// Debounce puts a job in the queue, in time
func Debounce(hook webhooks.Ref) {
	webhooks.Hooks <- hook
}

var jobsTimersMux sync.Mutex
var debounceTimers = make(map[string]*time.Timer)

func debounce(hook *webhooks.Ref, runOpts *options.ServerConfig) {
	// lock access to 'debounceTimers' and 'jobs'
	//fmt.Println("DEBUG [0] wait for jobs and timers")
	jobsTimersMux.Lock()
	defer func() {
		//fmt.Println("DEBUG [0] release jobs and timers")
		jobsTimersMux.Unlock()
	}()
	if _, exists := Jobs[URLSafeRefID(hook.GetURLSafeRefID())]; exists {
		log.Printf("Job in progress, not debouncing %s", hook)
		return
	}

	refID := hook.GetRefID()
	timer, ok := debounceTimers[refID]
	if ok {
		log.Printf("Replacing previous debounce timer for %s", hook)
		timer.Stop()
	}
	// this will not cause a mutual lock because it is async
	debounceTimers[refID] = time.AfterFunc(runOpts.DebounceDelay, func() {
		//fmt.Println("DEBUG [1] wait for jobs and timers")
		jobsTimersMux.Lock()
		delete(debounceTimers, refID)
		jobsTimersMux.Unlock()

		debounced <- hook
		//fmt.Println("DEBUG [1] release jobs and timers")
	})
}

func getBacklogFilePath(baseDir string, hook *webhooks.Ref) (string, string, error) {
	baseDir, _ = filepath.Abs(baseDir)
	fileName := hook.RefName + ".json"
	fileDir := filepath.Join(baseDir, hook.RepoID)

	err := os.MkdirAll(fileDir, 0755)

	return fileDir, fileName, err
}

func saveBacklog(hook *webhooks.Ref, runOpts *options.ServerConfig) {
	repoDir, repoFile, err := getBacklogFilePath(runOpts.TmpDir, hook)
	if nil != err {
		log.Printf("[WARN] could not create backlog dir %s:\n%v", repoDir, err)
		return
	}

	f, err := ioutil.TempFile(repoDir, "tmp-*")
	if nil != err {
		log.Printf("[WARN] could not create backlog file %s:\n%v", f.Name(), err)
		return
	}

	b, _ := json.MarshalIndent(hook, "", "  ")
	if _, err := f.Write(b); nil != err {
		log.Printf("[WARN] could not write backlog file %s:\n%v", f.Name(), err)
		return
	}

	replace := false
	jobFile := filepath.Join(repoDir, repoFile)
	if _, err := os.Stat(jobFile); nil == err {
		replace = true
		_ = os.Remove(jobFile)
	}
	if err := os.Rename(f.Name(), jobFile); nil != err {
		log.Printf("[WARN] rename backlog failed:\n%v", err)
		return
	}

	if replace {
		log.Printf("[backlog] replace backlog for %s", hook.GetRefID())
	} else {
		log.Printf("[backlog] create backlog for %s", hook.GetRefID())
	}
}

func run(curHook *webhooks.Ref, runOpts *options.ServerConfig) {
	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()

	jobID := URLSafeRefID(curHook.GetURLSafeRefID())
	if _, exists := Jobs[jobID]; exists {
		log.Printf("Job already in progress: %s", curHook.GetRefID())
		return
	}

	repoDir, repoFile, _ := getBacklogFilePath(runOpts.TmpDir, curHook)
	jobFile := filepath.Join(repoDir, repoFile)

	// TODO add mutex (should not affect temp files)
	_ = os.Remove(jobFile + ".cur")
	_ = os.Rename(jobFile, jobFile+".cur")
	b, err := ioutil.ReadFile(jobFile + ".cur")
	if nil != err {
		if !os.IsNotExist(err) {
			log.Printf("[warn] could not read backlog file %s:\n%v", repoFile, err)
		}
		// doesn't exist => no backlog
		log.Printf("[NO BACKLOG] no backlog for %s", repoFile)
		return
	}

	hook := &webhooks.Ref{}
	if err := json.Unmarshal(b, hook); nil != err {
		log.Printf("[warn] could not parse backlog file %s:\n%v", repoFile, err)
		return
	}
	hook = webhooks.New(*hook)

	env := os.Environ()
	envs := getEnvs(runOpts.Addr, string(jobID), runOpts.RepoList, hook)
	envs = append(envs, "GIT_DEPLOY_JOB_ID="+string(jobID))

	scriptPath, _ := filepath.Abs(runOpts.ScriptsPath + "/deploy.sh")
	args := []string{
		"-i",
		"--",
		//strings.Join([]string{
		scriptPath,
		string(jobID),
		hook.RefName,
		hook.RefType,
		hook.Owner,
		hook.Repo,
		hook.HTTPSURL,
		//}, " "),
	}

	args2 := append([]string{"[" + hook.RepoID + "#" + hook.RefName + "]", "bash"}, args...)
	fmt.Println(strings.Join(args2, " "))
	cmd := exec.Command("bash", args...)
	cmd.Env = append(env, envs...)

	now := time.Now()
	j := &Job{
		StartedAt: now,
		Cmd:       cmd,
		GitRef:    hook,
		Logs:      []Log{},
		Promote:   false,
	}
	// TODO NewJob()
	// Sets cmd.Stdout and cmd.Stderr
	f := setOutput(runOpts.LogDir, j)

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	Jobs[jobID] = j

	go func() {
		log.Printf("Started job for %s", hook)
		if err := cmd.Wait(); nil != err {
			log.Printf("gitdeploy job for %s#%s exited with error: %v", hook.HTTPSURL, hook.RefName, err)
		} else {
			log.Printf("gitdeploy job for %s#%s finished\n", hook.HTTPSURL, hook.RefName)
		}
		if nil != f {
			_ = f.Close()
		}

		// replace the text log with a json log
		if f, err := getJobFile(runOpts.LogDir, j.GitRef, ".json"); nil != err {
			// f.Name() should be the full path
			log.Printf("[warn] could not create log file '%s': %v", runOpts.LogDir, err)
		} else {
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if err := enc.Encode(j); nil != err {
				log.Printf("[warn] could not encode json log '%s': %v", f.Name(), err)
			} else {
				dpath, fpath, _ := getJobFilePath(runOpts.LogDir, j.GitRef, ".log")
				_ = os.Remove(filepath.Join(dpath, fpath))
			}
			_ = f.Close()
		}

		// waits for job to be declared completely dead
		deathRow <- jobID

		// debounces without saving in the backlog
		// TODO move this into deathRow?
		debacklog <- hook
	}()
}
