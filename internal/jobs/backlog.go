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
	// save to backlog
	saveBacklog(hook, runOpts)

	// lock access to 'debounceTimers' and 'jobs'
	fmt.Println("DEBUG [0] wait for jobs and timers")
	jobsTimersMux.Lock()
	defer func() {
		fmt.Println("DEBUG [0] release jobs and timers")
		jobsTimersMux.Unlock()
	}()
	if _, exists := Jobs[getJobID(hook)]; exists {
		log.Printf("[runHook] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
		return
	}
	refID := hook.GetRefID()
	timer, ok := debounceTimers[refID]
	if ok {
		timer.Stop()
	}
	// this will not cause a mutual lock because it is async
	timer = time.AfterFunc(2*time.Second, func() {
		fmt.Println("DEBUG [1] wait for jobs and timers")
		jobsTimersMux.Lock()
		defer jobsTimersMux.Unlock()
		restoreBacklog(hook, runOpts)
		delete(debounceTimers, refID)
		fmt.Println("DEBUG [1] release jobs and timers")
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

	jobFile := filepath.Join(repoDir, repoFile)
	if _, err := os.Stat(jobFile); nil == err {
		log.Printf("[backlog] remove stale job for %s", hook.GetRefID())
		_ = os.Remove(jobFile)
	}
	if err := os.Rename(f.Name(), jobFile); nil != err {
		log.Printf("[WARN] rename backlog failed:\n%v", err)
		return
	}

	log.Printf("[backlog] add fresh job for %s", hook.GetRefID())
}

func restoreBacklog(curHook *webhooks.Ref, runOpts *options.ServerConfig) {
	jobID := getJobID(curHook)
	if _, exists := Jobs[jobID]; exists {
		log.Printf("[runHook] gitdeploy debounced job for %s\n", curHook.GetRefID())
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
			log.Printf("[warn] could not create backlog file %s:\n%v", repoFile, err)
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

	log.Printf("[BACKLOG] pop backlog for %s", repoFile)
	env := os.Environ()
	envs := getEnvs(runOpts.Addr, jobID, runOpts.RepoList, hook)
	envs = append(envs, "GIT_DEPLOY_JOB_ID="+jobID)

	scriptPath, _ := filepath.Abs(runOpts.ScriptsPath + "/deploy.sh")
	args := []string{
		"-i",
		"--",
		//strings.Join([]string{
		scriptPath,
		jobID,
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

	j := &HookJob{
		ID:        jobID,
		Cmd:       cmd,
		GitRef:    hook,
		CreatedAt: hook.Timestamp,
		Logs:      []Log{},
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
		log.Printf("gitdeploy job for %s#%s started\n", hook.HTTPSURL, hook.RefName)
		if err := cmd.Wait(); nil != err {
			log.Printf("gitdeploy job for %s#%s exited with error: %v", hook.HTTPSURL, hook.RefName, err)
		} else {
			log.Printf("gitdeploy job for %s#%s finished\n", hook.HTTPSURL, hook.RefName)
		}
		if nil != f {
			_ = f.Close()
		}
		// nokill=true meaning let job finish? old cruft?
		fmt.Println("DEBUG load killers")
		//jobsTimersMux.Lock()
		//removeJob(jobID /*, true*/)
		//jobsTimersMux.Unlock()
		deathRow <- jobID
		fmt.Println("DEBUG load unkillers")
		//restoreBacklog(hook, runOpts)
		// re-debounce rather than running right away
		fmt.Println("DEBUG load Hook")
		webhooks.Hook(*hook)
		fmt.Println("DEBUG unload Hook")
	}()
}
