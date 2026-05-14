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
	Children map[string]fs.FS
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
	file, err := f.Overlay.Open(name)
	switch {
	default:
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  errors.Wrapf(err, "couldn't open file %s in overlay", name),
		}
	case errors.Is(err, fs.ErrNotExist):
		ref, ok := f.underlayRefs[name]
		if !ok {
			if !f.impliedDirs.Has(name) {
				return nil, &fs.PathError{
					Op:   "open",
					Path: name,
					Err:  errors.Errorf("file %s not found in either overlay or underlay", name),
				}
			}
			// TODO: implement this
			return nil, errors.New("unimplemented: opening file for implied dir")
		}
		file, err := ref.FS.Open(ref.Path)
		if err != nil {
			return nil, &fs.PathError{
				Op:   "open",
				Path: name,
				Err: errors.Wrapf(
					err, "couldn't open file %s in underlay as %s", name, path.Join(ref.FS.Path(), ref.Path),
				),
			}
		}
		return file, nil
	case err == nil:
		return file, nil
	}
}

// findChildContaining returns the child which contains the specified path. If multiple children
// contain the specified path, the child with the longest subdirectory path is returned.
func (f *UnionFS) findChildContaining(path string) (childPath string, child fs.FS, err error) {
	for p, c := range f.Children {
		if p does not contain path {
			continue
		}
		if len(p) > len(childPath) {
			childPath = p
			child = c
		}
	}

	if childPath == ""{
		return "", nil, errors.Errorf("couldn't find %s", path)
	}
	return childPath, child, nil
}

// Sub returns a UnionFS corresponding to the subtree rooted at dir.
func (f *UnionFS) Sub(dir string) (PathedFS, error) {
	dir = path.Clean(dir)
	// fmt.Printf("Sub(%s|%s)\n", f.Path(), dir)
	if dir == "." {
		return f, nil
	}
	overlaySub, err := f.Overlay.Sub(dir)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't make subtree for overlay")
	}

	prefix := dir + "/"
	underlayRefsSub := make(map[string]FileRef)
	for target, ref := range f.underlayRefs {
		if !strings.HasPrefix(target, prefix) {
			continue
		}
		underlayRefsSub[strings.TrimPrefix(target, prefix)] = ref
		// fmt.Printf("  - %s\n", strings.TrimPrefix(target, prefix))
	}
	return NewMergeFS(overlaySub, underlayRefsSub), nil
}

// UnionFS: fs.ReadDirFS

// ReadDir reads the named directory and returns a list of directory entries sorted by filename.
func (f *UnionFS) ReadDir(name string) (entries []fs.DirEntry, err error) {
	name = path.Clean(name)
	// fmt.Printf("ReadDir(%s|%s)\n", f.Path(), name)
	entryNames := make(structures.Set[string])

	info, err := fs.Stat(f.Overlay, name)
	if err == nil {
		if !info.IsDir() {
			return nil, &fs.PathError{
				Op:   "read",
				Path: name,
				Err:  errors.Wrapf(err, "%s is a non-directory file in overlay", name),
			}
		}
		if entries, err = fs.ReadDir(f.Overlay, name); err != nil {
			return nil, &fs.PathError{
				Op:   "read",
				Path: name,
				Err:  errors.Wrapf(err, "couldn't read directory %s in overlay", name),
			}
		}
		for _, entry := range entries {
			entryNames.Add(entry.Name())
		}
	}

	for target, ref := range f.underlayRefs {
		entryName := path.Base(target)
		if entryNames.Has(entryName) { // e.g. entry was already added by the overlay
			continue
		}
		entry, err := matchUnderlayRef(name, target, ref)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			continue
		}

		entries = append(entries, entry)
		entryNames.Add(entryName)
	}

	for dir := range f.impliedDirs {
		if path.Dir(dir) != name {
			continue
		}
		entryName := path.Base(dir)
		if entryNames.Has(entryName) { // e.g. entry was already added by the overlay
			continue
		}
		entries = append(entries, &impliedDirEntry{name: entryName})
		entryNames.Add(entryName)
	}

	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Name(), b.Name())
	})
	return entries, nil
}

// UnionFS: fs.ReadFileFS

// ReadFile returns the contents from reading the named file from the overlay (if it exists in the
// overlay), or else from an underlay filesystem depending on which one is recorded to have that
// file.
func (f *UnionFS) ReadFile(name string) ([]byte, error) {
	name = path.Clean(name)
	// fmt.Printf("ReadFile(%s|%s)\n", f.Path(), name)
	contents, err := fs.ReadFile(f.Overlay, name)
	switch {
	default:
		return nil, errors.Wrapf(err, "couldn't read file %s in overlay", name)
	case errors.Is(err, fs.ErrNotExist):
		ref, ok := f.underlayRefs[name]
		if !ok {
			if f.impliedDirs.Has(name) {
				return nil, &fs.PathError{
					Op:   "read",
					Path: name,
					Err:  errors.Errorf("file %s is a directory implied by the underlay", name),
				}
			}
			return nil, errors.Errorf(
				"file %s not found in either overlay or underlay of %s", name, f.Path(),
			)
		}
		contents, err := fs.ReadFile(ref.FS, ref.Path)
		return contents, errors.Wrapf(
			err, "couldn't read file %s in underlay as %s", name, path.Join(ref.FS.Path(), ref.Path),
		)
	case err == nil:
		return contents, nil
	}
}

// UnionFS: fs.StatFS

// Stat returns a [fs.FileInfo] describing the file from the overlay (if it exists in the overlay),
// or else from an underlay filesystem depending on which one is recorded to have that file.
func (f *UnionFS) Stat(name string) (fs.FileInfo, error) {
	name = path.Clean(name)
	// fmt.Printf("Stat(%s|%s)\n", f.Path(), name)
	info, err := fs.Stat(f.Overlay, name)
	switch {
	default:
		return nil, &fs.PathError{
			Op:   "stat",
			Path: name,
			Err:  errors.Wrapf(err, "couldn't stat file %s in overlay", name),
		}
	case errors.Is(err, fs.ErrNotExist):
		if name == "." {
			return &impliedDirFileInfo{name: path.Base(f.Path())}, nil
		}
		ref, ok := f.underlayRefs[name]
		if !ok {
			if !f.impliedDirs.Has(name) {
				return nil, &fs.PathError{
					Op:   "stat",
					Path: name,
					Err:  errors.Errorf("file %s not found in either overlay or underlay", name),
				}
			}
			// fmt.Printf("  %s is an implied dir!\n", name)
			return &impliedDirFileInfo{name: path.Base(name)}, nil
		}
		info, err := fs.Stat(ref.FS, ref.Path)
		if err != nil {
			return nil, &fs.PathError{
				Op:   "stat",
				Path: name,
				Err: errors.Wrapf(
					err, "couldn't stat file %s in underlay as %s", name, path.Join(ref.FS.Path(), ref.Path),
				),
			}
		}
		return info, nil
	case err == nil:
		return info, nil
	}
}

// UnionFS: fs.ReadLinkFS

func (f *UnionFS) ReadLink(name string) (string, error) {
	name = path.Clean(name)
	// fmt.Printf("ReadLink(%s|%s)\n", f.Path(), name)
	target, err := ReadLink(f.Overlay, name)
	switch {
	default:
		return "", &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err: errors.Wrapf(
				err, "couldn't stat (without following symlinks) file %s in overlay", name,
			),
		}
	case errors.Is(err, fs.ErrNotExist):
		ref, ok := f.underlayRefs[name]
		if !ok {
			return "", &fs.PathError{
				Op:   "lstat",
				Path: name,
				Err:  errors.Errorf("file %s not a symlink in overlay or underlay", name),
			}
		}
		target, err := ReadLink(ref.FS, ref.Path)
		if err != nil {
			return "", &fs.PathError{
				Op:   "lstat",
				Path: name,
				Err: errors.Wrapf(
					err, "couldn't stat file (without following symlinks) %s in underlay as %s",
					name, path.Join(ref.FS.Path(), ref.Path),
				),
			}
		}
		return target, nil
	case err == nil:
		return target, nil
	}
}

func (f *UnionFS) StatLink(name string) (fs.FileInfo, error) {
	name = path.Clean(name)
	// fmt.Printf("StatLink(%s|%s)\n", f.Path(), name)
	info, err := StatLink(f.Overlay, name)
	switch {
	default:
		return nil, &fs.PathError{
			Op:   "lstat",
			Path: name,
			Err: errors.Wrapf(
				err, "couldn't stat (without following symlinks) file %s in overlay", name,
			),
		}
	case errors.Is(err, fs.ErrNotExist):
		if name == "." {
			return &impliedDirFileInfo{name: path.Base(f.Path())}, nil
		}
		ref, ok := f.underlayRefs[name]
		if !ok {
			if !f.impliedDirs.Has(name) {
				return nil, &fs.PathError{
					Op:   "lstat",
					Path: name,
					Err:  errors.Errorf("file %s not found in either overlay or underlay", name),
				}
			}
			// fmt.Printf("  %s is an implied dir!\n", name)
			return &impliedDirFileInfo{name: path.Base(name)}, nil
		}
		info, err := StatLink(ref.FS, ref.Path)
		if err != nil {
			return nil, &fs.PathError{
				Op:   "lstat",
				Path: name,
				Err: errors.Wrapf(
					err, "couldn't stat file (without following symlinks) %s in underlay as %s",
					name, path.Join(ref.FS.Path(), ref.Path),
				),
			}
		}
		return info, nil
	case err == nil:
		return info, nil
	}
}
