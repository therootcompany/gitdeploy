package vfscopy

import (
	"io"
	"os"
	"path/filepath"
)

const (
	// tmpPermissionForDirectory makes the destination directory writable,
	// so that stuff can be copied recursively even if any original directory is NOT writable.
	// See https://github.com/otiai10/copy/pull/9 for more information.
	tmpPermissionForDirectory = os.FileMode(0755)
)

// CopyAll copies src to dest, doesn't matter if src is a directory or a file.
func CopyAll(vfs FileSystem, src, dest string, opt ...Options) error {
	// FYI: os.Open does a proper lstat
	f, err := vfs.Open(src)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	return switchboard(vfs, src, dest, f, info, assure(opt...))
}

// switchboard switches proper copy functions regarding file type, etc...
// If there would be anything else here, add a case to this switchboard.
func switchboard(
	vfs FileSystem, src, dest string, f File, info os.FileInfo, opt Options,
) error {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		// TODO
		return onsymlink(vfs, src, dest, opt)
	case info.IsDir():
		return dcopy(vfs, src, dest, f, info, opt)
	default:
		return fcopy(vfs, src, dest, f, info, opt)
	}
}

// copy decide if this src should be copied or not.
// Because this "copy" could be called recursively,
// "info" MUST be given here, NOT nil.
func copy(vfs FileSystem, src, dest string, f File, info os.FileInfo, opt Options) error {
	skip, err := opt.Skip(src)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	return switchboard(vfs, src, dest, f, info, opt)
}

// fcopy is for just a file,
// with considering existence of parent directory
// and file permission.
func fcopy(vfs FileSystem, src, dest string, f File, info os.FileInfo, opt Options) (err error) {

	if err = os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return
	}

	df, err := os.Create(dest)
	if err != nil {
		return
	}
	defer fclose(df, &err)

	if err = os.Chmod(df.Name(), info.Mode()|opt.AddPermission); err != nil {
		return
	}

	s, err := vfs.Open(src)
	if err != nil {
		return
	}
	defer fclose(s, &err)

	if _, err = io.Copy(df, s); err != nil {
		return
	}

	if opt.Sync {
		err = df.Sync()
	}

	return
}

// dcopy is for a directory,
// with scanning contents inside the directory
// and pass everything to "copy" recursively.
func dcopy(vfs FileSystem, srcdir, destdir string, d File, info os.FileInfo, opt Options) (err error) {

	originalMode := info.Mode()

	// Make dest dir with 0755 so that everything writable.
	if err = os.MkdirAll(destdir, tmpPermissionForDirectory); err != nil {
		return
	}
	// Recover dir mode with original one.
	defer chmod(destdir, originalMode|opt.AddPermission, &err)

	fileInfos, err := d.Readdir(-1)
	if err != nil {
		return
	}

	for _, newInfo := range fileInfos {
		cs, cd := filepath.Join(
			srcdir, newInfo.Name()),
			filepath.Join(destdir, newInfo.Name())

		f, err := vfs.Open(cs)
		if nil != err {
			return err
		}
		if err := copy(vfs, cs, cd, f, newInfo, opt); err != nil {
			// If any error, exit immediately
			return err
		}
	}

	return
}

func onsymlink(vfs FileSystem, src, dest string, opt Options) error {
	switch opt.OnSymlink(src) {
	case Shallow:
		return lcopy(vfs, src, dest)
	/*
	case Deep:
		orig, err := vfs.EvalSymlinks(src)
		if err != nil {
			return err
		}
		f, err := vfs.Open(orig)
		if err != nil {
			return err
		}
		//info, err := os.Lstat(orig)
		info, err := f.Stat()
		if err != nil {
			return err
		}
		return copy(vfs, orig, dest, f, info, opt)
	*/
	case Skip:
		fallthrough
	default:
		return nil // do nothing
	}
}

// lcopy is for a symlink,
// with just creating a new symlink by replicating src symlink.
func lcopy(vfs FileSystem, src, dest string) error {
	src, err := vfs.Readlink(src)
	if err != nil {
		return err
	}

	// Create the directories on the path to the dest symlink.
	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return err
	}

	return os.Symlink(src, dest)
}

// fclose ANYHOW closes file,
// with asiging error raised during Close,
// BUT respecting the error already reported.
func fclose(f File, reported *error) {
	if err := f.Close(); *reported == nil {
		*reported = err
	}
}

// chmod ANYHOW changes file mode,
// with asiging error raised during Chmod,
// BUT respecting the error already reported.
func chmod(dir string, mode os.FileMode, reported *error) {
	if err := os.Chmod(dir, mode); *reported == nil {
		*reported = err
	}
}

// assure Options struct, should be called only once.
// All optional values MUST NOT BE nil/zero after assured.
func assure(opts ...Options) Options {
	if len(opts) == 0 {
		return getDefaultOptions()
	}
	defopt := getDefaultOptions()
	if opts[0].OnSymlink == nil {
		opts[0].OnSymlink = defopt.OnSymlink
	}
	if opts[0].Skip == nil {
		opts[0].Skip = defopt.Skip
	}
	return opts[0]
}
