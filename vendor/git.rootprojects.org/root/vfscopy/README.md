# [vfscopy](https://git.rootprojects.org/root/vfscopy)

Recursively copy a Virtual FileSystem, such as
[http.FileSystem](https://golang.org/pkg/net/http/#FileSystem),
to a native file system destination.

Works with any file system that implements http.FileSystem,
including `vfsgen`, `fileb0x`, `gobindata` and most others.

## Example: native file system (os)

```go
httpfs := http.Dir("/tmp/public/")
vfs := vfscopy.NewVFS(httpfs)

if err := vfscopy.CopyAll(vfs, ".", "/tmp/dst/"); nil != err {
    fmt.Fprintf(os.Stderr, "couldn't copy vfs: %v\n", err)
}
```

## Example: vfsgen

**Note**: `vfsgen` does not support symlinks or file permissions.

```go
package main

import (
    "fmt"

    "git.rootprojects.org/root/vfscopy"

    // vfsgen-generated file system
    "git.example.com/org/project/assets"
)

func main() {
    vfs := vfscopy.NewVFS(assets.Assets)

    if err := vfscopy.CopyAll(vfs, ".", "/tmp/dst/"); nil != err {
        fmt.Fprintf(os.Stderr, "couldn't copy vfs: %v\n", err)
    }
    fmt.Println("Done.")
}
```

## Test

```bash
# Generate the test virtual file system
go generate ./...

# Run the tests
go test ./...
```

# License

The MIT License (MIT)

We used the recursive native file system copy implementation at
https://github.com/otiai10/copy as a starting point and added
virtual file system support.
