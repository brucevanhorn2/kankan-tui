package board

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// Watcher notifies on changes to a single board file. It watches the
// containing directory rather than the file itself, because atomic
// save patterns (write to a temp file, then rename over the target)
// replace the file's inode — a watch on the file alone would go stale
// after the first external save.
type Watcher struct {
	fsw    *fsnotify.Watcher
	path   string
	Events chan struct{}
	Errors chan error
}

func NewWatcher(path string) (*Watcher, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsw.Add(filepath.Dir(abs)); err != nil {
		fsw.Close()
		return nil, err
	}

	w := &Watcher{
		fsw:    fsw,
		path:   abs,
		Events: make(chan struct{}, 1),
		Errors: make(chan error, 1),
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) loop() {
	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			name, err := filepath.Abs(ev.Name)
			if err != nil || name != w.path {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			select {
			case w.Events <- struct{}{}:
			default:
			}
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			select {
			case w.Errors <- err:
			default:
			}
		}
	}
}

func (w *Watcher) Close() error {
	return w.fsw.Close()
}
