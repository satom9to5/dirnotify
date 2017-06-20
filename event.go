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
	WriteComplete
	Move = Create | Rename
)

type Op uint32

type Event struct {
	Op
	fi         *fileinfo.FileInfo
	beforePath string
}

type Events []Event

func flagString(op Op) string {
	flags := []string{}
	switch true {
	case op&Move == Move:
		flags = append(flags, "Move")
	case op&Create == Create:
		flags = append(flags, "Create")
	case op&Remove == Remove:
		flags = append(flags, "Remove")
	case op&Rename == Rename:
		flags = append(flags, "Rename")
	case op&Write == Write:
		flags = append(flags, "Write")
	case op&Chmod == Chmod:
		flags = append(flags, "Chmod")
	case op&WriteComplete == WriteComplete:
		flags = append(flags, "WriteComplete")
	}

	return strings.Join(flags, "|")
}

func (e Event) String() string {
	result := fmt.Sprintf("Op: %s", flagString(e.Op))

	if e.fi != nil {
		result += ", fi: " + e.fi.String()
	}
	if e.beforePath != "" {
		result += ", beforePath: " + e.beforePath
	}

	return result
}

func (e *Event) Name() (string, error) {
	if e.fi == nil {
		return "", errors.New("Name error: FileInfo is nil.")
	} else {
		return e.fi.Name(), nil
	}
}

func (e *Event) Size() (int64, error) {
	if e.fi == nil {
		return 0, errors.New("Size error: FileInfo is nil.")
	} else {
		return e.fi.Size(), nil
	}
}

func (e *Event) IsDir() (bool, error) {
	if e.fi == nil {
		return false, errors.New("IsDir error: FileInfo is nil.")
	} else {
		return e.fi.IsDir(), nil
	}
}

func (e *Event) Dir() (string, error) {
	if e.fi == nil {
		return "", errors.New("Dir error: FileInfo is nil.")
	} else {
		return e.fi.Dir(), nil
	}
}

func (e *Event) Path() (string, error) {
	if e.fi == nil {
		return "", errors.New("Path error: FileInfo is nil.")
	} else {
		return e.fi.Path(), nil
	}
}

func (e *Event) Ino() (uint64, error) {
	if e.fi == nil {
		return 0, errors.New("Info error: FileInfo is nil.")
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

	//fmt.Println("[Events/Add] eventQueue: " + q.String())

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
		r.RemoveNode(q.node)
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
	}

	ino, err := event.Ino()
	if err != nil {
		return err
	}

	// find same inode event.
	target := -1
	for index, e := range *es {
		eino, err := e.Ino()
		if err != nil {
			return err
		}

		if ino == eino {
			target = index
			break
		}
	}

	// merge same inode event. (pattern is Move only.)
	if target >= 0 {
		targetEvent := (*es)[target]

		switch true {
		case targetEvent.Op&Create == Create:
			switch true {
			case event.Op&Rename == Rename:
				event.beforePath = q.Path()
				if err != nil {
					return err
				}

				event.fi = targetEvent.FileInfo()
			}

			event.Op |= targetEvent.Op
		case targetEvent.Op&Rename == Rename:
			switch true {
			case event.Op&Create == Create:
				event.beforePath, err = targetEvent.Path()
				if err != nil {
					return err
				}
			}

			event.Op |= targetEvent.Op
		}

		(*es)[target] = event
	} else {
		*es = append(*es, event)
	}

	return nil
}
