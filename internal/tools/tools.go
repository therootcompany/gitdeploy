// +build tools

// Package tools is a faux package for tracking dependencies that don't make it into the code
package tools

import (
	// these are 'go generate' tooling dependencies, not included in the binary
	_ "github.com/shurcooL/vfsgen"
	_ "github.com/shurcooL/vfsgen/cmd/vfsgendev"
	_ "git.rootprojects.org/root/go-gitver/v2"
)
