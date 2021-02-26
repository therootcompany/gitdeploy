package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"git.rootprojects.org/root/gitdeploy/internal/api"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage go run pytest-to-gitdeploy.go ./pytest/.result.json ./converted.json\n")
		os.Exit(1)
		return
	}
	jsonpath := os.Args[1]
	reportpath := os.Args[2]

	b, err := ioutil.ReadFile(jsonpath)
	if nil != err {
		fmt.Fprintf(os.Stderr, "bad file path %s: %v\n", jsonpath, err)
		os.Exit(1)
		return
	}

	pyresult := api.PyResult{}
	if err := json.Unmarshal(b, &pyresult); nil != err {
		fmt.Fprintf(os.Stderr, "couldn't parse json %s: %v\n", string(b), err)
		os.Exit(1)
	}

	report := api.PyResultToReport(pyresult)
	b, _ = json.MarshalIndent(&report, "", "  ")

	if err := ioutil.WriteFile(reportpath, b, 0644); nil != err {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v", string(b), err)
		os.Exit(1)
	}
}
