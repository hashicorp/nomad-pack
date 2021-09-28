package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func walk(root string, walkFn filepath.WalkFunc) error {
	info, err := os.Lstat(root)
	if err != nil {
		err = walkFn(root, nil, err)
	} else {
		err = symwalk(root, info, walkFn)
	}
	if err == filepath.SkipDir {
		return nil
	}
	return err
}

func symwalk(path string, info os.FileInfo, walkFn filepath.WalkFunc) error {
	// Recursively walk symlinked directories.
	if isSymlink(info) {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("error evaluating symlink: %v", err)
		}
		if info, err = os.Lstat(resolved); err != nil {
			return err
		}
		if err := symwalk(path, info, walkFn); err != nil && err != filepath.SkipDir {
			return err
		}
		return nil
	}

	if err := walkFn(path, info, nil); err != nil {
		return err
	}

	if !info.IsDir() {
		return nil
	}

	names, err := readDirNames(path)
	if err != nil {
		return walkFn(path, info, err)
	}

	for _, name := range names {
		filename := filepath.Join(path, name)
		fileInfo, err := os.Lstat(filename)
		if err != nil {
			if err := walkFn(filename, fileInfo, err); err != nil && err != filepath.SkipDir {
				return err
			}
		} else {
			err = symwalk(filename, fileInfo, walkFn)
			if err != nil {
				if (!fileInfo.IsDir() && !isSymlink(fileInfo)) || err != filepath.SkipDir {
					return err
				}
			}
		}
	}
	return nil
}

func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, f.Close()
}

func isSymlink(fi os.FileInfo) bool {
	return fi.Mode()&os.ModeSymlink != 0
}
