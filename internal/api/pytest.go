package api

import (
	"fmt"

	"git.rootprojects.org/root/gitdeploy/internal/jobs"
)

// PyResultToReport converts from pytest's result.json to the GitDeploy report format
func PyResultToReport(pyresult PyResult) WrappedReport {
	report := jobs.Result{
		Name:    "pytest",
		Status:  "failed",
		Message: fmt.Sprintf("Exited with status code %d", pyresult.ExitCode),
		Detail:  pyresult,
	}

	var failed bool
	for i := range pyresult.Tests {
		unit := pyresult.Tests[i]
		report.Results = append(report.Results, jobs.Result{
			Name:   unit.NodeID,
			Status: unit.Outcome,
			//Detail: unit,
		})
		if "passed" != unit.Outcome {
			failed = true
		}
	}
	if !failed {
		report.Status = "passed"
	}

	return WrappedReport{
		Report: report,
	}
}

// PyResult is the pytest report
type PyResult struct {
	Created  float64 `json:"created"`
	Duration float64 `json:"duration"`
	ExitCode int     `json:"exitcode"`
	Root     string  `json:"root"`
	/*
		Environment struct {
			Python   string `json:"Python"`
			Platform string `json:"Platform"`
			Packages struct {
				Pytest string `json:"pytest"`
				Py     string `json:"py"`
				Pluggy string `json:"pluggy"`
			} `json:"Packages"`
			Plugins struct {
				HTML       string `json:"html"`
				Metadata   string `json:"metadata"`
				JSONReport string `json:"json-report"`
			} `json:"Plugins"`
		} `json:"environment"`
		Summary struct {
			Passed    int `json:"passed"`
			Total     int `json:"total"`
			Collected int `json:"collected"`
		} `json:"summary"`
		Collectors []struct {
			Nodeid  string `json:"nodeid"`
			Outcome string `json:"outcome"`
			Result  []struct {
				Nodeid string `json:"nodeid"`
				Type   string `json:"type"`
			} `json:"result"`
		} `json:"collectors"`
	*/
	Tests []struct {
		NodeID   string   `json:"nodeid"`
		LineNo   int      `json:"lineno"`
		Outcome  string   `json:"outcome"`
		Keywords []string `json:"keywords"`
		Setup    struct {
			Duration float64 `json:"duration"`
			Outcome  string  `json:"outcome"`
		} `json:"setup"`
		Call struct {
			Duration float64 `json:"duration"`
			Outcome  string  `json:"outcome"`
		} `json:"call"`
		Teardown struct {
			Duration float64 `json:"duration"`
			Outcome  string  `json:"outcome"`
		} `json:"teardown"`
	} `json:"tests"`
}

/*
{
  "created": 1614248016.458921,
  "duration": 16.896488904953003,
  "exitcode": 0,
  "root": "/home/app/srv/status.example.com/e2e-selenium",
  "environment": {
    "Python": "3.9.1",
    "Platform": "Linux-5.4.0-65-generic-x86_64-with-glibc2.31",
    "Packages": {
      "pytest": "6.2.1",
      "py": "1.10.0",
      "pluggy": "0.13.1"
    },
    "Plugins": {
      "html": "3.1.1",
      "metadata": "1.11.0",
      "json-report": "1.2.4"
    }
  },
  "summary": {
    "passed": 3,
    "total": 3,
    "collected": 3
  },
  "collectors": [
    {
      "nodeid": "",
      "outcome": "passed",
      "result": [
        {
          "nodeid": "test_landing_200.py",
          "type": "Module"
        }
      ]
    },
    {
      "nodeid": "test_landing_200.py",
      "outcome": "passed",
      "result": [
        {
          "nodeid": "test_landing_200.py::test_welcome_page_loads",
          "type": "Function",
          "lineno": 35
        },
        {
          "nodeid": "test_landing_200.py::test_create_account",
          "type": "Function",
          "lineno": 49
        },
        {
          "nodeid": "test_landing_200.py::test_login_existing",
          "type": "Function",
          "lineno": 85
        }
      ]
    }
  ],
  "tests": [
    {
      "nodeid": "test_landing_200.py::test_welcome_page_loads",
      "lineno": 35,
      "outcome": "passed",
      "keywords": [
        "e2e-selenium",
        "test_landing_200.py",
        "test_welcome_page_loads"
      ],
      "setup": {
        "duration": 0.0006089679664000869,
        "outcome": "passed"
      },
      "call": {
        "duration": 2.512254447909072,
        "outcome": "passed"
      },
      "teardown": {
        "duration": 0.00038311502430588007,
        "outcome": "passed"
      }
    }
  ]
}
*/
