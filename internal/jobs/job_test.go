package jobs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"
)

var debounceDelay time.Duration
var jobDelay time.Duration
var runOpts *options.ServerConfig
var logDir string

func init() {
	tmpDir, _ := ioutil.TempDir("", "gitdeploy-*")
	runOpts = &options.ServerConfig{
		Addr:          "localhost:4483",
		ScriptsPath:   "./testdata",
		LogDir:        "./test-logs",
		TmpDir:        tmpDir,
		DebounceDelay: 25 * time.Millisecond,
		StaleAge:      5 * time.Minute,
		//StaleAge:      50000 * time.Millisecond,
	}
	logDir, _ = filepath.Abs(runOpts.LogDir)

	os.Setenv("GIT_DEPLOY_TEST_WAIT", "0.1")
	debounceDelay = 50 * time.Millisecond
	jobDelay = 250 * time.Millisecond
}

func TestDebounce(t *testing.T) {
	t.Log("TestDebounce Log Dir: " + logDir)

	Start(runOpts)

	t0 := time.Now()

	t1 := t0.Add(-100 * time.Second)
	t2 := t0.Add(-90 * time.Second)
	t3 := t0.Add(-80 * time.Second)
	t4 := t0.Add(-70 * time.Second)
	t5 := t0.Add(-60 * time.Second)

	r1 := "abcdef7890"
	r2 := "1abcdef789"
	r3 := "12abcdef78"
	r4 := "123abcdef7"
	r5 := "1234abcdef"

	// skip debounce
	Debounce(webhooks.Ref{
		Timestamp: t1,
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       r1,
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	})
	// skip debounce
	Debounce(webhooks.Ref{
		Timestamp: t2,
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       r2,
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	})
	// hit
	Debounce(webhooks.Ref{
		Timestamp: t3,
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       r3,
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	})

	// TODO make debounce time configurable
	t.Log("sleep so job can debounce and start")
	time.Sleep(debounceDelay)

	var jobMatch *Job
	all := All()
	for i := range all {
		// WARN: lock value copied
		j := all[i]
		fmt.Printf("[TEST] Job[%d]: %#v", i, *j.GitRef)
		if t0 == j.GitRef.Timestamp ||
			t1 == j.GitRef.Timestamp ||
			t2 == j.GitRef.Timestamp ||
			r1 == j.GitRef.Rev ||
			r2 == j.GitRef.Rev {
			t.Error(fmt.Errorf("should not find debounced jobs"))
			t.Fail()
			return
		}
		if t3 == j.GitRef.Timestamp || r3 == j.GitRef.Rev {
			if nil != jobMatch {
				t.Error(fmt.Errorf("should find only one instance of the 1st long-standing job"))
				t.Fail()
				return
			}
			jobMatch = all[i]
		}
	}
	if nil == jobMatch {
		t.Error(fmt.Errorf("should find the 1st long-standing job"))
		t.Fail()
		return
	}

	t.Log("put another job on the queue while job is running")
	// backlog debounce
	Debounce(webhooks.Ref{
		Timestamp: t4,
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       r4,
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	})
	// backlog hit
	Debounce(webhooks.Ref{
		Timestamp: t5,
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       r5,
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	})

	t.Log("sleep so 1st job can finish")
	time.Sleep(jobDelay)
	time.Sleep(jobDelay)
	t.Log("sleep so backlog can debounce")
	time.Sleep(debounceDelay)

	//var j *Job
	jobMatch = nil
	all = All()
	for i := range all {
		j := all[i]
		fmt.Printf("[TEST] Job[%d]: %#v", i, *j.GitRef)
		if t4 == j.GitRef.Timestamp ||
			r4 == j.GitRef.Rev {
			t.Error(fmt.Errorf("should not find debounced jobs"))
			t.Fail()
			return
		}
		if t5 == j.GitRef.Timestamp || r5 == j.GitRef.Rev {
			if nil != jobMatch {
				t.Error(fmt.Errorf("should find only one instance of the 2nd long-standing job"))
				t.Fail()
				return
			}
			jobMatch = all[i]
		}
	}
	if nil == jobMatch {
		t.Error(fmt.Errorf("should find the 2nd long-standing job"))
		t.Fail()
		return
	}

	t.Log("sleep so 2nd job can finish")
	time.Sleep(jobDelay)

	t.Log("sleep to ensure no more backlogs exist")
	time.Sleep(jobDelay)
	time.Sleep(debounceDelay)
	time.Sleep(debounceDelay)

	Stop()
}

func TestActive(t *testing.T) {
}

func TestRecent(t *testing.T) {
}

func TestStale(t *testing.T) {
}
