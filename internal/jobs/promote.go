package jobs

import (
	"os"
	"os/exec"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/log"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

// Promotions channel
var Promotions = make(chan Promotion)

// Promotion is a channel message
type Promotion struct {
	PromoteTo string
	GitRef    *webhooks.Ref
}

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
	jobID1 := hook.GetRefID()
	hookTo := *hook
	hookTo.RefName = promoteTo
	jobID2 := hookTo.GetRefID()

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

	if _, ok := Actives.Load(jobID1); ok {
		// TODO put promote in backlog
		log.Printf("[promote] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, hook.RefName)
		return
	}
	if _, ok := Actives.Load(jobID2); ok {
		// TODO put promote in backlog
		log.Printf("[promote] gitdeploy job already started for %s#%s\n", hook.HTTPSURL, promoteTo)
		return
	}

	if err := cmd.Start(); nil != err {
		log.Printf("gitdeploy exec error: %s\n", err)
		return
	}

	t := time.Now()
	now := &t
	promoteID := hook.RepoID + "#" + hook.RefName + ".." + promoteTo
	Actives.Store(jobID1, &Job{
		StartedAt: now,
		ID:        promoteID,
		GitRef:    hook,
		PromoteTo: promoteTo,
		Promote:   true, // deprecated
		cmd:       cmd,
	})
	Actives.Store(jobID2, &Job{
		StartedAt: now,
		ID:        promoteID,
		GitRef:    hook,
		PromoteTo: promoteTo,
		Promote:   true, // deprecated
		cmd:       cmd,
	})
	Actives.Store(promoteID, &Job{
		StartedAt: now,
		ID:        promoteID,
		GitRef:    hook,
		PromoteTo: promoteTo,
		Promote:   true, // deprecated
		cmd:       cmd,
	})

	go func() {
		log.Printf("gitdeploy promote for %s#%s started\n", hook.HTTPSURL, hook.RefName)
		_ = cmd.Wait()
		deathRow <- jobID1
		deathRow <- jobID2
		deathRow <- webhooks.RefID(promoteID)
		log.Printf("gitdeploy promote for %s#%s finished\n", hook.HTTPSURL, hook.RefName)
		// TODO check for backlog
	}()
}
