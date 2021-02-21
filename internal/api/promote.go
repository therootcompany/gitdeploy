package api

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

func runPromote(hook webhooks.Ref, promoteTo string, runOpts *options.ServerConfig) {
	// TODO create an origin-branch tag with a timestamp?
	jobID1 := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.HTTPSURL, hook.RefName),
	))
	jobID2 := base64.RawURLEncoding.EncodeToString([]byte(
		fmt.Sprintf("%s#%s", hook.HTTPSURL, promoteTo),
	))

	args := []string{
		runOpts.ScriptsPath + "/promote.sh",
		jobID1,
		promoteTo,
		hook.RefName,
		hook.RefType,
		hook.Owner,
		hook.Repo,
		hook.HTTPSURL,
	}
	cmd := exec.Command("bash", args...)

	env := os.Environ()
	envs := getEnvs(runOpts.Addr, jobID1, runOpts.RepoList, hook)
	envs = append(envs, "GIT_DEPLOY_PROMOTE_TO="+promoteTo)
	cmd.Env = append(env, envs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if _, exists := jobs[jobID1]; exists {
		// TODO put promote in backlog
		log.Printf("[promote] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
		return
	}
	if _, exists := jobs[jobID2]; exists {
		// TODO put promote in backlog
		log.Printf("[promote] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, promoteTo)
		return
	}

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	jobs[jobID1] = &HookJob{
		ID:        jobID2,
		Cmd:       cmd,
		GitRef:    hook,
		CreatedAt: time.Now(),
	}
	jobs[jobID2] = &HookJob{
		ID:        jobID2,
		Cmd:       cmd,
		GitRef:    hook,
		CreatedAt: time.Now(),
	}

	go func() {
		log.Printf("gitdeploy promote for %s#%s started\n", hook.HTTPSURL, hook.RefName)
		_ = cmd.Wait()
		killers <- jobID1
		killers <- jobID2
		log.Printf("gitdeploy promote for %s#%s finished\n", hook.HTTPSURL, hook.RefName)
		// TODO check for backlog
	}()
}
