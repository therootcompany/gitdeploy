package gitea

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"git.ryanburnette.com/ryanburnette/git-deploy/internal/options"
	"git.ryanburnette.com/ryanburnette/git-deploy/internal/webhooks"

	"github.com/go-chi/chi"
)

func init() {
	var secret string
	name := "gitea"
	options.ServerFlags.StringVar(
		&secret, fmt.Sprintf("%s-secret", name), "",
		fmt.Sprintf(
			"secret for %s webhooks (same as %s_SECRET=)",
			name, strings.ToUpper(name)),
	)
	webhooks.AddProvider("gitea", InitWebhook("gitea", &secret, "GITEA_SECRET"))
}

// InitWebhook prepares the webhook router.
// It should be called after arguments are parsed and ENVs are set.InitWebhook
func InitWebhook(providername string, secret *string, envname string) func() {
	return func() {
		if "" == *secret {
			*secret = os.Getenv(envname)
		}
		if "" == *secret {
			fmt.Fprintf(os.Stderr, "skipped route for missing %s\n", envname)
			return
		}
		secretB := []byte(*secret)
		webhooks.AddRouteHandler(providername, func(router chi.Router) {
			router.Post("/", func(w http.ResponseWriter, r *http.Request) {
				r.Body = http.MaxBytesReader(w, r.Body, options.DefaultMaxBodySize)

				payload, err := ioutil.ReadAll(r.Body)
				if err != nil {
					// if there's a read error, it should have been handled already by the MaxBytesReader
					return
				}

				sig := r.Header.Get("X-Gitea-Signature")
				sigB, err := hex.DecodeString(sig)
				if !ValidMAC(payload, sigB, secretB) {
					log.Printf("invalid gitea signature: %q\n", sig)
					http.Error(w, "invalid gitea signature", http.StatusBadRequest)
					return
				}

				info := Webhook{}
				if err := json.Unmarshal(payload, &info); nil != err {
					log.Printf("invalid gitea payload: error: %s\n%s\n", err, string(payload))
					http.Error(w, "invalid gitea payload", http.StatusBadRequest)
					return
				}

				var tag string
				var branch string
				ref := info.Ref // refs/heads/master
				parts := strings.Split(ref, "/")
				refType := parts[1] // refs/[heads]/master
				prefixLen := len("refs/") + len(refType) + len("/")
				refName := ref[prefixLen:]
				switch refType {
				case "tags":
					refType = "tag"
					tag = refName
				case "heads":
					refType = "branch"
					branch = refName
				default:
					refType = "unknown"
				}

				webhooks.Hook(webhooks.Ref{
					HTTPSURL: info.Repository.CloneURL,
					SSHURL:   info.Repository.SSHURL,
					Rev:      info.After,
					Ref:      ref,
					RefType:  refType,
					RefName:  refName,
					Branch:   branch,
					Tag:      tag,
					Repo:     info.Repository.Name,
					Owner:    info.Repository.Owner.Login,
				})
			})
		})
	}
}

// ValidMAC reports whether messageMAC is a valid HMAC tag for message.
func ValidMAC(message, messageMAC, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}
