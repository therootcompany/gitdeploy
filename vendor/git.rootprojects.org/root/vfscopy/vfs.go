package vfscopy

import (
	"os"
	"io"
	"io/ioutil"
	"path/filepath"
	"path"
	"strings"
	"errors"
	"net/http"
)

// FileSystem is a Virtual FileSystem with Symlink support
type FileSystem interface {
    Open(name string) (File, error)
	Readlink(name string) (string, error)
	//EvalSymlinks(name string) (string, error)
}

// File is copied from http.File
type File interface {
    io.Closer
    io.Reader
    io.Seeker
    Readdir(count int) ([]os.FileInfo, error)
    Stat() (os.FileInfo, error)
}

// VFS is a virtual FileSystem with Symlink support
type VFS struct {
	FileSystem http.FileSystem
}

// Open opens a file relative to a virtual filesystem
func (v *VFS) Open(name string) (File, error) {
	return v.FileSystem.Open(name)
}

// Readlink returns a "not implemented" error,
// which is okay because it is never called for http.FileSystem.
func (v *VFS) Readlink(name string) (string, error) {
	f, err := v.FileSystem.Open(name)
	if nil != err {
		return "", err
	}
	b, err := ioutil.ReadAll(f)
	if nil != err {
		return "", err
	}
	return string(b), nil
}

// NewVFS gives an http.FileSystem (real) symlink support
func NewVFS(httpfs http.FileSystem) FileSystem {
	return &VFS{ FileSystem: httpfs }
}

// Dir is an implementation of a Virtual FileSystem
type Dir string

// mapDirOpenError maps the provided non-nil error from opening name
// to a possibly better non-nil error. In particular, it turns OS-specific errors
// about opening files in non-directories into os.ErrNotExist. See Issue 18984.
func mapDirOpenError(originalErr error, name string) error {
	if os.IsNotExist(originalErr) || os.IsPermission(originalErr) {
		return originalErr
	}

	parts := strings.Split(name, string(filepath.Separator))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		fi, err := os.Stat(strings.Join(parts[:i+1], string(filepath.Separator)))
		if err != nil {
			return originalErr
		}
		if !fi.IsDir() {
			return os.ErrNotExist
		}
	}
	return originalErr
}

func (d Dir) fullName(name string) (string, error) {
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return "", errors.New("http: invalid character in file path")
	}
	dir := string(d)
	if dir == "" {
		dir = "."
	}
	fullName := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	return fullName, nil
}

// Open opens a file relative to a virtual filesystem
func (d Dir) Open(name string) (File, error) {
	fullName, err := d.fullName(name)
	if nil != err {
		return nil, err
	}
	f, err := os.Open(fullName)
	if err != nil {
		return nil, mapDirOpenError(err, fullName)
	}
	return f, nil
}

// Readlink returns the destination of the named symbolic link.
func (d Dir) Readlink(name string) (string, error) {
	name, err := d.fullName(name)
	if nil != err {
		return "", err
	}
	name, err = os.Readlink(name)
	if err != nil {
		return "", mapDirOpenError(err, name)
	}
	return name, nil
}

/*
// EvalSymlinks returns the destination of the named symbolic link.
func (d Dir) EvalSymlinks(name string) (string, error) {
	name, err := d.fullName(name)
	if nil != err {
		return "", err
	}
	name, err = filepath.EvalSymlinks(name)
	if err != nil {
		return "", mapDirOpenError(err, name)
	}
	return name, nil
}
*/
