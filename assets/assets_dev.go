// +build dev

package assets

import "net/http"

// Assets is the public file system which should be served by http
var Assets http.FileSystem = http.Dir("../public")
