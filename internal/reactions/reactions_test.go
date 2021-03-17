package reactions

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"git.rootprojects.org/root/gitdeploy/internal/jobs"
	"git.rootprojects.org/root/gitdeploy/internal/webhooks"

	"github.com/joho/godotenv"
)

type Ref struct {
	RepoID    string    `json:"repo_id"`
	Timestamp time.Time `json:"timestamp"`
	HTTPSURL  string    `json:"https_url"`
	SSHURL    string    `json:"ssh_url"`
	Rev       string    `json:"rev"`
	Ref       string    `json:"ref"`      // refs/tags/v0.0.1, refs/heads/master
	RefType   string    `json:"ref_type"` // tag, branch
	RefName   string    `json:"ref_name"`
	Owner     string    `json:"repo_owner"`
	Repo      string    `json:"repo_name"`
	//Branch    string    `json:"branch"` // deprecated
	//Tag       string    `json:"tag"`    // deprecated
}

func TestMain(m *testing.M) {
	godotenv.Load("../../.env.test")
	m.Run()
}

func parseEnv(envs []string) map[string]string {
	menvs := map[string]string{}
	for _, env := range envs {
		index := strings.Index(env, "=")
		// this allows empty string as a key
		if index > -1 {
			menvs[env[:index]] = env[index+1:]
		}
	}
	return menvs
}

type Renderable struct {
	Config map[string]interface{}
	Env    map[string]string
	Report *jobs.Result
	Hook   *webhooks.Ref
}

func TestMailgun(t *testing.T) {
	f, err := os.Open("./testdata/action.json")
	if nil != err {
		t.Error(err)
		return
	}
	config := &[]Config{}

	dec := json.NewDecoder(f)
	if err := dec.Decode(config); nil != err {
		t.Error(err)
		return
	}
	fmt.Printf("\n%#v\n\n", *config)

	j := &jobs.Job{
		GitRef: webhooks.New(webhooks.Ref{
			HTTPSURL: "https://github.com/example/project.git",
			RefName:  "twig",
		}),
		Report: &jobs.Result{
			Status: "bar",
		},
	}
	body, err := encodeBody((*config)[0].Notifications[0], Renderable{
		//Config: (*config)[0].Notifications[0].Config,
		Config: (*config)[0].Matchers[0].Config,
		Env:    parseEnv(os.Environ()),
		Hook:   j.GitRef,
		Report: j.Report,
	})
	if nil != err {
		fmt.Printf("%v\n", err)
	}

	fmt.Printf("%s\n", body)
}
