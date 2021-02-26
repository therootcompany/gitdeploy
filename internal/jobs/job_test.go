package jobs

import (
	"encoding/base64"
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

var t0 = time.Now().UTC()

func init() {
	tmpDir, _ := ioutil.TempDir("", "gitdeploy-*")
	runOpts = &options.ServerConfig{
		Addr:          "localhost:4483",
		ScriptsPath:   "./testdata",
		LogDir:        "./test-logs/debounce",
		TmpDir:        tmpDir,
		DebounceDelay: 25 * time.Millisecond,
		StaleJobAge:   5 * time.Minute,
		StaleLogAge:   5 * time.Minute,
		ExpiredLogAge: 10 * time.Minute,
	}
	logDir, _ = filepath.Abs(runOpts.LogDir)

	os.Setenv("GIT_DEPLOY_TEST_WAIT", "0.1")
	debounceDelay = 50 * time.Millisecond
	jobDelay = 250 * time.Millisecond

	Start(runOpts)
}

func TestDebounce(t *testing.T) {
	t.Log("TestDebounce Log Dir: " + logDir)

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

	t.Log("sleep so job can debounce and start")
	time.Sleep(debounceDelay)

	var jobMatch *Job
	all := All(time.Time{})
	for i := range all {
		// WARN: lock value copied
		j := all[i]
		//t.Logf("[TEST] A-Job[%d]: %s\n%#v\n", i, j.GitRef.Timestamp, *j.GitRef)
		if t0.Equal(j.GitRef.Timestamp) ||
			(t1.Equal(j.GitRef.Timestamp) && r1 == j.GitRef.Rev) ||
			(t2.Equal(j.GitRef.Timestamp) && r2 == j.GitRef.Rev) {
			t.Errorf("should not find debounced jobs")
			t.Fail()
			return
		}
		if t3.Equal(j.GitRef.Timestamp) && r3 == j.GitRef.Rev {
			if nil != jobMatch {
				t.Errorf("should find only one instance of the 1st long-standing job")
				t.Fail()
				return
			}
			jobMatch = all[i]
		}
	}
	if nil == jobMatch {
		t.Errorf("should find the 1st long-standing job")
		t.Fail()
		return
	}

	//t.Log("put another job on the queue while job is running")
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
	all = All(time.Time{})
	for i := range all {
		j := all[i]
		//t.Logf("[TEST] B-Job[%d]: %s\n%#v\n", i, j.GitRef.Timestamp, *j.GitRef)
		if t4.Equal(j.GitRef.Timestamp) && r4 == j.GitRef.Rev {
			t.Errorf("should not find debounced jobs")
			t.Fail()
			return
		}
		if t5.Equal(j.GitRef.Timestamp) && r5 == j.GitRef.Rev {
			if nil != jobMatch {
				t.Errorf("should find only one instance of the 2nd long-standing job")
				t.Fail()
				return
			}
			jobMatch = all[i]
		}
	}
	if nil == jobMatch {
		t.Errorf("should find the 2nd long-standing job: %s %s", t5, r5)
		t.Fail()
		return
	}

	t.Log("sleep so 2nd job can finish")
	time.Sleep(jobDelay)

	t.Log("sleep to ensure no more backlogs exist")
	time.Sleep(jobDelay)
	time.Sleep(debounceDelay)
	time.Sleep(debounceDelay)

	//Stop()
}

func TestRecents(t *testing.T) {
	/*
		tmpDir, _ := ioutil.TempDir("", "gitdeploy-*")
		runOpts = &options.ServerConfig{
			Addr:          "localhost:4483",
			ScriptsPath:   "./testdata",
			LogDir:        "./test-logs/recents",
			TmpDir:        tmpDir,
			DebounceDelay: 1 * time.Millisecond,
			StaleJobAge:   5 * time.Minute,
			StaleLogAge:   5 * time.Minute,
			ExpiredLogAge: 10 * time.Minute,
		}
		logDir, _ = filepath.Abs(runOpts.LogDir)

		os.Setenv("GIT_DEPLOY_TEST_WAIT", "0.01")
		debounceDelay := 50 * time.Millisecond
		jobDelay := 250 * time.Millisecond
	*/

	//Start(runOpts)

	t6 := t0.Add(-50 * time.Second)
	r6 := "12345abcde"

	// skip debounce
	hook := webhooks.Ref{
		Timestamp: t6,
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       r6,
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	}
	Debounce(hook)

	t.Log("sleep so job can debounce and start")
	time.Sleep(debounceDelay)
	time.Sleep(jobDelay)
	t.Log("sleep so job can finish")
	time.Sleep(jobDelay)

	urlRefID := webhooks.URLSafeGitID(
		base64.RawURLEncoding.EncodeToString([]byte(hook.GetRefID())),
	)
	j, err := LoadLogs(runOpts, urlRefID)
	if nil != err {
		urlRevID := webhooks.URLSafeGitID(
			base64.RawURLEncoding.EncodeToString([]byte(hook.GetRevID())),
		)
		j, err = LoadLogs(runOpts, urlRevID)
		if nil != err {
			t.Errorf("error loading logs: %v", err)
			return
		}
		return
	}

	if len(j.Logs) < 3 {
		t.Errorf("should have logs from test deploy script: %#v", j.Logs)
		t.Fail()
		return
	}

	if nil == j.ExitCode || 0 != *j.ExitCode {
		t.Errorf("should zero exit status")
		t.Fail()
		return
	}

	//t.Logf("[DEBUG] Report:\n%#v", j.Report)

	//t.Logf("Logs:\n%v", err)

	//Stop()
}
