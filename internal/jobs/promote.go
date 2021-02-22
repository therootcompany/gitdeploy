package jobs

import (
	"os"
	"os/exec"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

// Promote will run the promote script
func Promote(msg webhooks.Ref, promoteTo string) {
	Promotions <- Promotion{
		PromoteTo: promoteTo,
		GitRef:    &msg,
	}
}

// promote will run the promote script
func promote(hook *webhooks.Ref, promoteTo string, runOpts *options.ServerConfig) {
	// TODO create an origin-branch tag with a timestamp?
	jobID1 := URLSafeRefID(hook.GetURLSafeRefID())
	hookTo := *hook
	hookTo.RefName = promoteTo
	jobID2 := URLSafeRefID(hookTo.GetURLSafeRefID())

	args := []string{
		runOpts.ScriptsPath + "/promote.sh",
		string(jobID1),
		promoteTo,
		hook.RefName,
		hook.RefType,
		hook.Owner,
		hook.Repo,
		hook.HTTPSURL,
	}
	cmd := exec.Command("bash", args...)

	env := os.Environ()
	envs := getEnvs(runOpts.Addr, string(jobID1), runOpts.RepoList, hook)
	envs = append(envs, "GIT_DEPLOY_PROMOTE_TO="+promoteTo)
	cmd.Env = append(env, envs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if _, exists := Jobs[jobID1]; exists {
		// TODO put promote in backlog
		log.Printf("[promote] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
		return
	}
	if _, exists := Jobs[jobID2]; exists {
		// TODO put promote in backlog
		log.Printf("[promote] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, promoteTo)
		return
	}

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	now := time.Now()
	Jobs[jobID1] = &Job{
		StartedAt: now,
		Cmd:       cmd,
		GitRef:    hook,
		Promote:   true,
	}
	Jobs[jobID2] = &Job{
		StartedAt: now,
		Cmd:       cmd,
		GitRef:    hook,
		Promote:   true,
	}

	go func() {
		log.Printf("gitdeploy promote for %s#%s started\n", hook.HTTPSURL, hook.RefName)
		_ = cmd.Wait()
		deathRow <- jobID1
		deathRow <- jobID2
		log.Printf("gitdeploy promote for %s#%s finished\n", hook.HTTPSURL, hook.RefName)
		// TODO check for backlog
	}()
}
