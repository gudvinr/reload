package reload

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
)

// WatchDirectories listens for changes in directories and
// broadcasts on write.
func (reload *Reloader) WatchDirectories() {
	if len(reload.directories) == 0 {
		reload.Logger.Error("no directories provided (reload.Directories is empty)")
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		reload.Logger.Error("error initializing fsnotify watcher", "err", err)
	}

	defer w.Close()

	for _, path := range reload.directories {
		directories, err := recursiveWalk(path)
		if err != nil {
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				path = pathErr.Path
			}
			reload.Logger.Error("error walking directories", "path", path, "err", err)
			return
		}

		for _, dir := range directories {
			// Path is converted to absolute path, so that fsnotify.Event also contains
			// absolute paths
			absPath, err := filepath.Abs(dir)
			if err != nil {
				reload.Logger.Error("Failed to convert path to absolute path", "err", err)
				continue
			}
			w.Add(absPath)
		}
	}

	reload.Logger.Info("watching for changes", "directories", reload.directories)

	debounce := debounce.New(100 * time.Millisecond)

	callback := func(path string) func() {
		return func() {
			reload.Logger.Debug("edit", "path", path)
			if reload.OnReload != nil {
				reload.OnReload()
			}
			reload.cond.Broadcast()
		}
	}

	for {
		select {
		case err := <-w.Errors:
			reload.Logger.Error("error watching", "err", err)
		case e := <-w.Events:
			switch {
			case e.Has(fsnotify.Create):
				dir := filepath.Dir(e.Name)
				// Watch any created directory
				if err := w.Add(dir); err != nil {
					reload.Logger.Error("error watching", "name", e.Name, "err", err)
					continue
				}
				debounce(callback(path.Base(e.Name)))

			case e.Has(fsnotify.Write):
				debounce(callback(path.Base(e.Name)))

			case e.Has(fsnotify.Rename), e.Has(fsnotify.Remove):
				// a renamed file might be outside the specified paths
				directories, _ := recursiveWalk(e.Name)
				for _, v := range directories {
					w.Remove(v)
				}
				w.Remove(e.Name)
			}
		}
	}
}

func recursiveWalk(path string) ([]string, error) {
	var res []string
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			res = append(res, path)
		}
		return nil
	})

	return res, err
}
