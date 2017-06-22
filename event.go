package dirnotify

import (
	"errors"
	"fmt"
	"strings"
	// third party
	"github.com/satom9to5/fileinfo"
)

const (
	Create Op = 1 << iota
	Remove
	Rename
	Write
	Chmod
	Move
	WriteComplete
)

type Op uint32

type Event struct {
	Op
	fi         *fileinfo.FileInfo
	beforePath string
}

type Events []Event

var (
	flagList = []struct {
		Op
		name string
	}{
		{Create, "Create"},
		{Remove, "Remove"},
		{Rename, "Rename"},
		{Write, "Write"},
		{Chmod, "Chmod"},
		{Move, "Move"},
		{WriteComplete, "WriteComplete"},
	}
)

func flagString(op Op) string {
	flags := []string{}

	for _, f := range flagList {
		if op&f.Op == f.Op {
			flags = append(flags, f.name)
		}
	}

	return strings.Join(flags, "|")
}

func (e Event) String() string {
	result := fmt.Sprintf("Op: %s", flagString(e.Op))

	if e.fi != nil {
		result += ", fi: {" + e.fi.String() + "}"
	}
	if e.beforePath != "" {
		result += ", beforePath: " + e.beforePath
	}

	return result
}

func (e *Event) Name() (string, error) {
	if e.fi == nil {
		return "", errors.New("[Event/Name] error: FileInfo is nil.")
	} else {
		return e.fi.Name(), nil
	}
}

func (e *Event) Size() (int64, error) {
	if e.fi == nil {
		return 0, errors.New("[Event/Size] error: FileInfo is nil.")
	} else {
		return e.fi.Size(), nil
	}
}

func (e *Event) IsDir() (bool, error) {
	if e.fi == nil {
		return false, errors.New("[Event/IsDir] error: FileInfo is nil.")
	} else {
		return e.fi.IsDir(), nil
	}
}

func (e *Event) Dir() (string, error) {
	if e.fi == nil {
		return "", errors.New("[Event/Dir] error: FileInfo is nil.")
	} else {
		return e.fi.Dir(), nil
	}
}

func (e *Event) Path() (string, error) {
	if e.fi == nil {
		return "", errors.New("[Event/Path] error: FileInfo is nil.")
	} else {
		return e.fi.Path(), nil
	}
}

func (e *Event) Ino() (uint64, error) {
	if e.fi == nil {
		return 0, errors.New("[Event/Ino] error: FileInfo is nil.")
	} else {
		return e.fi.Ino(), nil
	}
}

func (e *Event) FileInfo() *fileinfo.FileInfo {
	return e.fi
}

func (e *Event) BeforePath() string {
	return e.beforePath
}

// TODO update tree & quques when Rename / Remove Event
func (es *Events) Add(q eventQueue, eq *eventQueues, r *Root) error {
	event := Event{
		Op: q.Op,
	}
	var err error

	switch true {
	case q.Op&Create == Create:
		fi, err := fileinfo.Stat(q.Path())
		if err != nil {
			return err
		}

		event.fi = fi

		if node := r.InoFind(fi.Ino()); node != nil {
			// when same inode found

			// rename dir of eventQueues
			eq.Rename(node.Path(), fi.Path())

			// rename nodes
			if err = r.RenameNode(node, fi.Dir(), fi.Name()); err != nil {
				return err
			}
		} else {
			// add root
			node, err := r.CreateAddNode(fi.Path())
			if err != nil {
				return err
			}
			// append children dirs/files
			// example case is create directories on os.mkdirAll
			if node.IsDir() {
				if err = r.appendNodes(node); err != nil {
					return err
				}
			}
			// append eventQueues
			eq.AddFromNodes(node.children())
		}
	case q.Op&Remove == Remove:
		if q.node == nil {
			return errors.New("[Events/Add] Remove error: FileInfo is nil.")
		}

		// set remove node info
		event.fi = q.node.info
		// remove node
		if q.Path() == q.node.Path() {
			r.RemoveNode(q.node)
		}
	case q.Op&Rename == Rename:
		// TODO remove from watcher
		if q.node == nil {
			return errors.New("[Events/Add] Rename error: FileInfo is nil.")
		}

		// set rename node info
		event.fi = q.node.info

		// rename finished on Create Event.
		// delete current node on thie condition.

		// when fail rename
		if q.Path() == q.node.Path() {
			// don't happen rename function
			r.RemoveNode(q.node)
		}
	// TODO write code.
	case q.Op&Write == Write:
		return nil
	case q.Op&Chmod == Chmod:
		return nil
	}

	ino, err := event.Ino()
	if err != nil {
		return err
	}

	// find same inode event.
	targetI := -1
	for index, e := range *es {
		eino, err := e.Ino()
		if err != nil {
			return err
		}

		if ino == eino {
			targetI = index
			break
		}
	}

	if targetI == -1 {
		*es = append(*es, event)
		return nil
	}

	// merge same inode event. (pattern is Move only.)
	targetEvent := (*es)[targetI]

	switch true {
	case targetEvent.Op&Create == Create:
		switch true {
		case event.Op&Remove == Remove, event.Op&Rename == Rename:
			event.beforePath = q.Path()
			if err != nil {
				return err
			}

			event.fi = targetEvent.FileInfo()
		}

		event.Op |= targetEvent.Op
	case targetEvent.Op&Remove == Remove, targetEvent.Op&Rename == Rename:
		switch true {
		case event.Op&Create == Create:
			event.beforePath, err = targetEvent.Path()
			if err != nil {
				return err
			}
		}

		event.Op |= targetEvent.Op
	}

	(*es)[targetI] = event

	return nil
}

func (es *Events) UpdateOp() {
	for i, e := range *es {
		if e.Op&Create == Create && e.Op&(Remove|Rename) > 0 {
			e.Op = Move
			(*es)[i] = e
		}
	}
}
