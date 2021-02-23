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
var debounceTimers = make(map[webhooks.RefID]*time.Timer)

func debounce(hook *webhooks.Ref, runOpts *options.ServerConfig) {
	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()

	activeID := hook.GetRefID()
	if _, ok := Actives.Load(activeID); ok {
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
	pendingID := hook.GetRefID()
	Pending.Store(pendingID, hook)

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
	backlogPath := filepath.Join(repoDir, repoFile)
	if _, err := os.Stat(backlogPath); nil == err {
		replace = true
		_ = os.Remove(backlogPath)
	}
	if err := os.Rename(f.Name(), backlogPath); nil != err {
		log.Printf("[WARN] rename backlog json failed:\n%v", err)
		return
	}

	if replace {
		log.Printf("[backlog] replace backlog for %s", hook.GetRefID())
	} else {
		log.Printf("[backlog] create backlog for %s", hook.GetRefID())
	}
}

func run(curHook *webhooks.Ref, runOpts *options.ServerConfig) {
	// because we want to lock the whole transaction all of the state
	jobsTimersMux.Lock()
	defer jobsTimersMux.Unlock()

	pendingID := curHook.GetRefID()
	if _, ok := Actives.Load(pendingID); ok {
		log.Printf("Job already in progress: %s", curHook.GetRefID())
		return
	}

	var hook *webhooks.Ref
	// Legacy, but would be nice to repurpose for resuming on reload
	repoDir, repoFile, _ := getBacklogFilePath(runOpts.TmpDir, curHook)
	backlogFile := filepath.Join(repoDir, repoFile)
	if value, ok := Pending.Load(pendingID); ok {
		hook = value.(*webhooks.Ref)
		log.Printf("loaded from Pending state: %#v", hook)
	} else {
		// TODO add mutex (should not affect temp files)
		_ = os.Remove(backlogFile + ".cur")
		_ = os.Rename(backlogFile, backlogFile+".cur")
		b, err := ioutil.ReadFile(backlogFile + ".cur")
		if nil != err {
			if !os.IsNotExist(err) {
				log.Printf("[warn] could not read backlog file %s:\n%v", repoFile, err)
			}
			// doesn't exist => no backlog
			log.Printf("[NO BACKLOG] no backlog for %s", repoFile)
			return
		}

		hook = &webhooks.Ref{}
		if err := json.Unmarshal(b, hook); nil != err {
			log.Printf("[warn] could not parse backlog file %s:\n%v", repoFile, err)
			return
		}
		hook = webhooks.New(*hook)
		log.Printf("loaded from file: %#v", hook)
	}

	Pending.Delete(pendingID)
	_ = os.Remove(backlogFile)
	_ = os.Remove(backlogFile + ".cur")

	env := os.Environ()
	envs := getEnvs(runOpts.Addr, string(pendingID), runOpts.RepoList, hook)
	envs = append(envs, "GIT_DEPLOY_JOB_ID="+string(pendingID))

	scriptPath, _ := filepath.Abs(runOpts.ScriptsPath + "/deploy.sh")
	args := []string{
		"-i",
		"--",
		//strings.Join([]string{
		scriptPath,
		string(pendingID),
		hook.RefName,
		hook.RefType,
		hook.Owner,
		hook.Repo,
		hook.HTTPSURL,
		//}, " "),
	}

	args2 := append([]string{"[" + string(hook.GetRefID()) + "]", "bash"}, args...)
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
	// TODO jobs.New()
	// Sets cmd.Stdout and cmd.Stderr
	f := setOutput(runOpts.LogDir, j)

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	Actives.Store(pendingID, j)

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

		// Switch ID to the more specific RevID
		j.ID = string(j.GitRef.GetRevID())
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
				logdir, logname, _ := getJobFilePath(runOpts.LogDir, j.GitRef, ".log")
				_ = os.Remove(filepath.Join(logdir, logname))
			}
			_ = f.Close()
			log.Printf("[DEBUG] wrote log to %s", f.Name())
		}
		j.Logs = []Log{}

		// this will completely clear the finished job
		deathRow <- pendingID

		// debounces without saving in the backlog
		// TODO move this into deathRow?
		debacklog <- hook
	}()
}
