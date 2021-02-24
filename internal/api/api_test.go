package api

import (
	"fmt"
	"io/ioutil"
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
		ScriptsPath:   "./testdata",
		LogDir:        "./test-logs/debounce",
		TmpDir:        tmpDir,
		DebounceDelay: 25 * time.Millisecond,
		StaleJobAge:   5 * time.Minute,
		StaleLogAge:   5 * time.Minute,
		ExpiredLogAge: 10 * time.Minute,
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

		//dec := json.NewDecoder(r.Body)
		//dec.Decode()
	*/

	t.Log("sleep so job can debounce, start, and finish")
	time.Sleep(debounceDelay)
	time.Sleep(jobDelay)

	// TODO test that the API gives this back to us
	urlRevID := hook.GetURLSafeRevID()

	// TODO needs auth
	reqURL := fmt.Sprintf("http://%s/api/admin/logs/%s",
		runOpts.Addr,
		string(urlRevID),
	)
	resp, err := http.Get(reqURL)
	if nil != err {
		t.Logf("[DEBUG] Response Error: %v", err)
		return
	}

	t.Logf("[DEBUG] Request URL: %s", reqURL)
	t.Logf("[DEBUG] Response Headers: %d %#v", resp.StatusCode, resp.Header)
	b, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		t.Logf("[DEBUG] Response Error: %v", err)
		return
	}
	t.Logf("[DEBUG] Response Body: %v", string(b))
}
