package fs

import (
	"cmp"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/pkg/errors"

	"github.com/forklift-run/forklift/exp/structures"
)

// UnionFS

// A UnionFS is an FS constructed as a merger of other FSes, each of which is given its own
// subdirectory path within the UnionFS as the root directory.
type UnionFS struct {
	RootPath string
	Children map[string]PathedFS
}

// UnionFS: resolverFS

// Resolve returns the path of the named file from the appropriate child.
func (f *UnionFS) Resolve(name string) (string, error) {
	child, ok := f.Children[name]
	if ok {
		return child.Path(), nil
	}

	childPath, child, err := f.findChildContaining(name)
	if err != nil {
		return "", &fs.PathError{
			Op:   PathErrorOpResolve,
			Path: name,
			Err:  errors.Wrapf(err, "couldn't find a matching union child for %s", name),
		}
	}
	fsys, ok := child.(resolverFS)
	if !ok {
		return path.Join(child.Path(), childPath), nil
	}
	return fsys.Resolve(childPath)
}

// UnionFS: PathedFS

// Path returns the path of the overlay.
func (f *UnionFS) Path() string {
	return f.RootPath
}

// Open opens the named file from the matching child (if it exists).
func (f *UnionFS) Open(name string) (fs.File, error) {
	name = path.Clean(name)
	// fmt.Printf("Open(%s|%s)\n", f.Path(), name)
	child, ok := f.Children[name]
	if ok {
		return child.Open(".")
	}

	childPath, child, err := f.findChildContaining(name)
	if err != nil {
		return nil, &fs.PathError{
			Op:   PathErrorOpOpen,
			Path: name,
			Err:  errors.Wrapf(err, "couldn't find a matching union child for %s", name),
		}
	}
	return child.Open(childPath)
}

// findChildContaining returns the child which contains the specified path, and the subdirectory
// path of the specified path within the child. If multiple children
// contain the specified path, the child with the longest subdirectory path is returned.
func (f *UnionFS) findChildContaining(p string) (
	childPath string, child PathedFS, err error,
) {
	p = path.Clean(p)
	// fmt.Printf("findChildContaining %s\n", p)
	palletCandidatePath := p
	for {
		if child, ok := f.Children[palletCandidatePath]; ok {
			// fmt.Printf(
			// 	"  found: %s\n    %s\n", child.Path(), strings.TrimPrefix(p, palletCandidatePath+"/"),
			// )
			return strings.TrimPrefix(p, palletCandidatePath+"/"), child, nil
		}
		palletCandidatePath = path.Dir(palletCandidatePath)
		if palletCandidatePath == "/" || palletCandidatePath == "." {
			// we can't go up anymore!
			return "", nil, errors.Errorf("couldn't find %s", p)
		}
	}
}

// Sub returns a UnionFS corresponding to the subtree rooted at dir.
func (f *UnionFS) Sub(dir string) (PathedFS, error) {
	dir = path.Clean(dir)
	// fmt.Printf("Sub(%s|%s)\n", f.Path(), dir)
	if dir == "." {
		return f, nil
	}
	child, ok := f.Children[dir]
	if ok {
		return child, nil
	}

	childPath, child, err := f.findChildContaining(dir)
	if err != nil {
		return nil, &fs.PathError{
			Op:   PathErrorOpSub,
			Path: dir,
			Err:  errors.Wrapf(err, "couldn't find a matching union child for %s", dir),
		}
	}
	return child.Sub(childPath)
}

// UnionFS: fs.ReadDirFS

// ReadDir reads the named directory and returns a list of directory entries sorted by filename.
func (f *UnionFS) ReadDir(name string) (entries []fs.DirEntry, err error) {
	name = path.Clean(name)
	// fmt.Printf("ReadDir(%s|%s)\n", f.Path(), name)
	entryNames := make(structures.Set[string])

	if name == "." {
		for entryName := range f.Children {
			entries = append(entries, &impliedDirEntry{name: entryName})
			entryNames.Add(entryName)
		}
		slices.SortFunc(entries, func(a, b fs.DirEntry) int {
			return cmp.Compare(a.Name(), b.Name())
		})
		return entries, nil
	}

	child, ok := f.Children[name]
	if ok {
		return fs.ReadDir(child, ".")
	}

	childPath, child, err := f.findChildContaining(name)
	if err != nil {
		return nil, &fs.PathError{
			Op:   PathErrorOpReadDir,
			Path: name,
			Err:  errors.Wrapf(err, "couldn't find a matching union child for %s", name),
		}
	}
	return fs.ReadDir(child, childPath)
}

// UnionFS: fs.ReadFileFS

// ReadFile returns the contents from reading the named file in the matching child (if it exists in
// the child.
func (f *UnionFS) ReadFile(name string) ([]byte, error) {
	name = path.Clean(name)
	// fmt.Printf("ReadFile(%s|%s)\n", f.Path(), name)
	if name == "." {
		return nil, &fs.PathError{
			Op:   PathErrorOpRead,
			Path: name,
			Err:  errors.Errorf("file %s is the unionfs itself", name),
		}
	}
	if _, ok := f.Children[name]; ok {
		return nil, &fs.PathError{
			Op:   PathErrorOpRead,
			Path: name,
			Err:  errors.Errorf("file %s is a union child of the unionfs", name),
		}
	}

	childPath, child, err := f.findChildContaining(name)
	if err != nil {
		return nil, &fs.PathError{
			Op:   PathErrorOpRead,
			Path: name,
			Err:  errors.Wrapf(err, "couldn't find a matching union child for %s", name),
		}
	}
	return fs.ReadFile(child, childPath)
}

// UnionFS: fs.StatFS

// Stat returns a [fs.FileInfo] describing the file from the overlay (if it exists in the overlay),
// or else from an underlay filesystem depending on which one is recorded to have that file.
func (f *UnionFS) Stat(name string) (fs.FileInfo, error) {
	name = path.Clean(name)
	// fmt.Printf("Stat(%s|%s)\n", f.Path(), name)
	if name == "." {
		return &impliedDirFileInfo{name: path.Base(f.Path())}, nil
	}

	_, ok := f.Children[name]
	if ok {
		return &impliedDirFileInfo{name: name}, nil
	}

	childPath, child, err := f.findChildContaining(name)
	if err != nil {
		return nil, &fs.PathError{
			Op:   PathErrorOpOpen,
			Path: name,
			Err:  errors.Wrapf(err, "couldn't find a matching union child for %s", name),
		}
	}
	return fs.Stat(child, childPath)
}

// UnionFS: fs.ReadLinkFS

func (f *UnionFS) ReadLink(name string) (string, error) {
	name = path.Clean(name)
	// fmt.Printf("ReadLink(%s|%s)\n", f.Path(), name)
	if name == "." {
		return "", &fs.PathError{
			Op:   PathErrorOpLstat,
			Path: name,
			Err:  errors.Errorf("file %s is the unionfs itself", name),
		}
	}
	if _, ok := f.Children[name]; ok {
		return "", &fs.PathError{
			Op:   PathErrorOpLstat,
			Path: name,
			Err:  errors.Errorf("file %s is a union child of the unionfs", name),
		}
	}

	childPath, child, err := f.findChildContaining(name)
	if err != nil {
		return "", &fs.PathError{
			Op:   PathErrorOpLstat,
			Path: name,
			Err:  errors.Wrapf(err, "couldn't find a matching union child for %s", name),
		}
	}
	return fs.ReadLink(child, childPath)
}

func (f *UnionFS) StatLink(name string) (fs.FileInfo, error) {
	name = path.Clean(name)
	// fmt.Printf("StatLink(%s|%s)\n", f.Path(), name)
	if name == "." {
		return f.Stat(".")
	}

	_, ok := f.Children[name]
	if ok {
		return f.Stat(name)
	}

	childPath, child, err := f.findChildContaining(name)
	if err != nil {
		return nil, &fs.PathError{
			Op:   PathErrorOpLstat,
			Path: name,
			Err:  errors.Wrapf(err, "couldn't find a matching union child for %s", name),
		}
	}
	return fs.Lstat(child, childPath)
}
