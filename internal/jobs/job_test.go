package jobs

import (
	"path/filepath"
	"testing"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

func TestDebounce(t *testing.T) {
	runOpts := &options.ServerConfig{
		Addr:        "localhost:4483",
		ScriptsPath: "./testdata",
		LogDir:      "./test-logs",
	}

	logDir, _ := filepath.Abs(runOpts.LogDir)
	t.Log("test debounce: " + logDir)

	// TODO move to Init() in job.go?
	go func() {
		// TODO read from backlog
		for n := 0; n < 7; n++ {
			t.Logf("cycle %d", n)
			select {
			//hook := webhooks.Accept()
			case hook := <-webhooks.Hooks:
				t.Logf("start: hook accepted")
				debounce(webhooks.New(hook), runOpts)
				t.Logf("start: hook completed")
			case jobID := <-deathRow:
				t.Logf("kill: jobID accepted")
				// !nokill signifies that the job should be forcefully killed
				remove(jobID /*, false*/)
				t.Logf("kill: jobID removed")
			}
		}
	}()

	// skip debounce
	webhooks.Hooks <- webhooks.Ref{
		Timestamp: time.Now(),
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       "abcdef7890",
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	}
	// skip debounce
	webhooks.Hooks <- webhooks.Ref{
		Timestamp: time.Now(),
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       "1abcdef789",
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	}
	// hit
	webhooks.Hooks <- webhooks.Ref{
		Timestamp: time.Now(),
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       "12abcdef78",
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	}

	// TODO make debounce time configurable
	t.Log("sleep so job can debounce and start")
	time.Sleep(2100 * time.Millisecond)

	t.Log("put another job on the queue while job is running")
	// backlog debounce
	webhooks.Hooks <- webhooks.Ref{
		Timestamp: time.Now(),
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       "123abcdef7",
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	}
	// backlog hit
	webhooks.Hooks <- webhooks.Ref{
		Timestamp: time.Now(),
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       "1234abcdef",
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	}

	t.Log("sleep so 1st job can finish")
	time.Sleep(2100 * time.Millisecond)
	t.Log("sleep so backlog can debounce")
	time.Sleep(2100 * time.Millisecond)
	t.Log("sleep so 2nd job can finish")
	time.Sleep(2100 * time.Millisecond)

	t.Log("sleep to ensure no more backlogs exist")
	time.Sleep(2100 * time.Millisecond)

	close(webhooks.Hooks)
}
