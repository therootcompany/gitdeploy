// Package reactions generates web hooks to fire at other services
package reactions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"

	"git.rootprojects.org/root/golib/https"
)

// Config pairs Matchers to Notifications
type Config struct {
	Matchers      []Matcher      `json:"matchers"`
	Notifications []Notification `json:"notifications"`
}

// Notification is a dynamic web request
type Notification struct {
	Name    string                 `json:"name"`
	Method  string                 `json:"method"`
	URL     string                 `json:"url"`
	Auth    map[string]string      `json:"auth"`
	Headers map[string]string      `json:"headers"`
	Form    map[string]string      `json:"form"`
	JSON    map[string]string      `json:"json"`
	Config  map[string]interface{} `json:"config"`
	Configs []map[string]string    `json:"configs"`
}

type Matcher struct {
	Repo          string                 `json:"repo"`
	Branches      []string               `json:"branches"`
	Notifications []string               `json:"notifications"`
	Config        map[string]interface{} `json:"config"`
}

type Target struct {
	Matcher
	Env []string `json:"-"`
}

func Do(h Notification, data interface{}) error {
	body, err := encodeBody(h, data)
	if nil != err {
		return err
	}
	r := strings.NewReader(body)

	return doRequest(h, data, r)
}

func doRequest(h Notification, data interface{}, body io.Reader) error {
	client := https.NewHTTPClient()
	url, err := doTmpl("notification-url", "URL", h.URL, data)
	if nil != err {
		return err
	}

	req, err := http.NewRequest(h.Method, url, body)
	if nil != err {
		return err
	}

	if 0 != len(h.Form) {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else if 0 != len(h.JSON) {
		req.Header.Set("Content-Type", "application/json")
	}

	if 0 != len(h.Auth) {
		user := h.Auth["user"]
		if "" == user {
			user = h.Auth["username"]
		}
		user, err = doTmpl("notification-basic-username", "username", user, data)
		if nil != err {
			return err
		}

		pass := h.Auth["pass"]
		if "" == pass {
			pass = h.Auth["password"]
		}
		pass, err = doTmpl("notification-basic-username", "username", pass, data)
		if nil != err {
			return err
		}

		req.SetBasicAuth(user, pass)
	}

	req.Header.Set("User-Agent", "Reactions/0.1.0")
	for k := range h.Headers {
		req.Header.Set(k, h.Headers[k])
	}

	resp, err := client.Do(req)
	if nil != err {
		return err
	}

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return fmt.Errorf("response error for '%s': %s", h.Name, resp.Status)
	}

	// TODO json vs xml vs txt
	var result map[string]interface{}
	req.Header.Add("Accept", "application/json")
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		return fmt.Errorf("response body error for '%s': %s: %v", h.Name, resp.Status, err)
	}

	fmt.Printf("\n%#v\n", result)

	return nil
}

func doTmpl(name, short, tmpl string, data interface{}) (string, error) {
	t, err := template.New(name).Parse(tmpl)
	if nil != err {
		return "", fmt.Errorf("error parsing "+short+"template: %v\n%q\n", err, tmpl)
	}
	t = t.Option("missingkey=default")
	// strings.Builder{} is like a one-directional bytes.Buffer
	var w strings.Builder
	if err := t.Execute(&w, data); nil != err {
		return "", fmt.Errorf("error executing "+short+" template: %v\n", err)
	}
	return w.String(), nil
}

func encodeBody(h Notification, data interface{}) (string, error) {
	if "" == h.Method {
		h.Method = "POST"
	}

	if len(h.Form) > 0 {
		form := url.Values{}
		for k := range h.Form {
			v := h.Form[k]

			t, err := template.New("http-action-form").Parse(v)
			if nil != err {
				fmt.Fprintf(os.Stderr, "error parsing form field template: %v\n%q\n", err, v)
				continue
			}
			t = t.Option("missingkey=default")

			// strings.Builder{} is like a one-directional bytes.Buffer
			var w strings.Builder
			if err := t.Execute(&w, data); nil != err {
				fmt.Fprintf(os.Stderr, "error executing form field template: %v\n", err)
				continue
			}
			form.Set(k, w.String())
		}
		return form.Encode(), nil
	}

	if len(h.JSON) > 0 {
		// no error check because it's not possible for the JSON,
		// which was just recently parsed to suddenly have a circular reference
		bodyBuf, _ := json.Marshal(h.JSON)
		v := string(bodyBuf)

		t, err := template.New("http-action-json").Parse(v)
		if nil != err {
			return "", err
		}
		t = t.Option("missingkey=default")

		// strings.Builder{} is like a one-directional bytes.Buffer
		var w strings.Builder
		if err := t.Execute(&w, data); nil != err {
			return "", err
		}
		return v, nil
	}

	return "", nil
}
