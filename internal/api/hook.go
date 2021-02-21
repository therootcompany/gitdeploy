package api

import (
	"os/exec"
	"strings"
	"sync"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

// HookJob is a job started by the git webhook
type HookJob struct {
	ID        string // {HTTPSURL}#{BRANCH}
	Cmd       *exec.Cmd
	ExitCode  *int
	GitRef    webhooks.Ref
	CreatedAt time.Time
	Logs      []Log
	Report    JobReport
	mux       sync.Mutex
}

// JobReport should have many items
type JobReport struct {
	Items []string
}

// https://git.example.com/example/project.git
//      => git.example.com/example/project
func getRepoID(httpsURL string) string {
	repoID := strings.TrimPrefix(httpsURL, "https://")
	repoID = strings.TrimPrefix(repoID, "http://")
	repoID = strings.TrimSuffix(repoID, ".git")
	return repoID
}

func getTimestamp(t time.Time) time.Time {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t
}

// TODO NewRef
func normalizeHook(hook *webhooks.Ref) {
	hook.RepoID = getRepoID(hook.HTTPSURL)
	hook.Timestamp = getTimestamp(hook.Timestamp)
}
