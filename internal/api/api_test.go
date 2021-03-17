package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/jobs"
	"git.rootprojects.org/root/gitdeploy/internal/options"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"

	"github.com/go-chi/chi"
)

var server *httptest.Server
var runOpts *options.ServerConfig
var debounceDelay time.Duration
var jobDelay time.Duration
var logDir string

func init() {
	tmpDir, _ := ioutil.TempDir("", "gitdeploy-*")
	runOpts = &options.ServerConfig{
		//Addr:          "localhost:4483",
		ScriptsPath:       "./testdata",
		LogDir:            "./test-logs/api",
		TmpDir:            tmpDir,
		DebounceDelay:     25 * time.Millisecond,
		DefaultMaxJobTime: 5 * time.Second, // very short
		StaleJobAge:       5 * time.Minute,
		StaleLogAge:       5 * time.Minute,
		ExpiredLogAge:     10 * time.Minute,
	}
	logDir, _ = filepath.Abs(runOpts.LogDir)

	r := chi.NewRouter()
	server = httptest.NewServer(r)
	runOpts.Addr = server.Listener.Addr().String()
	RouteStopped(r, runOpts)

	os.Setenv("GIT_DEPLOY_TEST_WAIT", "0.1")
	debounceDelay = 50 * time.Millisecond
	jobDelay = 250 * time.Millisecond

	jobs.Start(runOpts)

	//server.Close()
}

func ToUnixSeconds(t time.Time) float64 {
	// 1614236182.651912345
	secs := float64(t.Unix())                          // 1614236182
	nanos := float64(t.Nanosecond()) / 1_000_000_000.0 // 0.651912345
	//nanos := (float64(t.UnixNano()) - secs) / 1_000_000_000.0 // 0.651912345

	// in my case I want to truncate the precision to milliseconds
	nanos = math.Round((10000 * nanos) / 10000) // 0.6519

	s := secs + nanos // 1614236182.651912345
	return s
}

func TestCallback(t *testing.T) {
	// TODO use full API request with local webhook
	t7 := time.Now().Add(-40 * time.Second)
	r7 := "ef1234abcd"

	// skip debounce
	hook := webhooks.Ref{
		Timestamp: t7,
		RepoID:    "git.example.com/owner/repo",
		HTTPSURL:  "https://git.example.com/owner/repo.git",
		Rev:       r7,
		RefName:   "master",
		RefType:   "branch",
		Owner:     "owner",
		Repo:      "repo",
	}
	// , _ := json.MarshallIndent(&hook , "", "  ")
	jobs.Debounce(hook)
	/*
		body := bytes.NewReader(hook)
		r := httptest.NewRequest("POST", "/api/local/webhook", body)
	*/

	// TODO use callback or chan chan to avoid sleeps?
	time.Sleep(debounceDelay)
	t.Log("sleep so job can finish")
	time.Sleep(jobDelay)
	time.Sleep(jobDelay)

	// TODO test that the API gives this back to us
	urlRevID := hook.GetURLSafeRevID()

	s := ToUnixSeconds(t7.Add(-1 * time.Second))

	// TODO needs auth
	reqURL := fmt.Sprintf(
		"http://%s/api/admin/logs/%s?since=%f",
		runOpts.Addr, string(urlRevID), s,
	)
	resp, err := http.Get(reqURL)
	if nil != err {
		t.Errorf("HTTP response error: %s\n%#v", reqURL, err)
		return
	}

	job := &jobs.Job{}
	b, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(b, job); nil != err {
		t.Errorf(
			"response decode error: %d %s\n%#v\n%#v",
			resp.StatusCode, reqURL, resp.Header, err,
		)
		return
	}

	if len(job.Logs) < 3 {
		t.Errorf("too few logs: %s\n%s\n%#v", reqURL, string(b), job)
		return
	}

	if nil == job.Report || len(job.Report.Results) < 1 {
		t.Errorf("too few results: %s\n%#v", reqURL, job)
		return
	}

	if nil == job.ExitCode || 0 != *job.ExitCode {
		t.Errorf("non-zero exit code: %s\n%#v", reqURL, job)
		return
	}
}
