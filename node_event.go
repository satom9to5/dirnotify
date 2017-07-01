package dirnotify

import (
	"errors"
	"fmt"
	// third party
	"github.com/satom9to5/fileinfo"
)

// for send channel

type nodeEvent struct {
	Op
	node       *Node
	beforePath string
}

type nodeEvents []nodeEvent

func (ne nodeEvent) String() string {
	result := fmt.Sprintf("Op: %s", flagString(ne.Op))

	if ne.node != nil {
		result += ", node: {" + ne.node.String() + "}"
	}
	if ne.beforePath != "" {
		result += ", beforePath: " + ne.beforePath
	}

	return result
}

func (ne *nodeEvent) Name() (string, error) {
	if ne.node == nil {
		return "", errors.New("[nodeEvent/Name] error: Node is nil.")
	} else {
		return ne.node.Name(), nil
	}
}

// called from nodeEvent.node.
func (ne *nodeEvent) Size() (int64, error) {
	if ne.node == nil {
		return 0, errors.New("[nodeEvent/Size] error: Node is nil.")
	} else {
		return ne.node.Size(), nil
	}
}

func (ne *nodeEvent) IsDir() (bool, error) {
	if ne.node == nil {
		return false, errors.New("[nodeEvent/IsDir] error: Node is nil.")
	} else {
		return ne.node.IsDir(), nil
	}
}

func (ne *nodeEvent) Dir() (string, error) {
	if ne.node == nil {
		return "", errors.New("[nodeEvent/Dir] error: Node is nil.")
	} else {
		return ne.node.Dir(), nil
	}
}

func (ne *nodeEvent) Path() (string, error) {
	if ne.node == nil {
		return "", errors.New("[nodeEvent/Path] error: Node is nil.")
	} else {
		return ne.node.Path(), nil
	}
}

func (ne *nodeEvent) Ino() (uint64, error) {
	if ne.node == nil {
		return 0, errors.New("[nodeEvent/Ino] error: Node is nil.")
	} else {
		return ne.node.Ino(), nil
	}
}

func (ne *nodeEvent) FileInfo() *fileinfo.FileInfo {
	if ne.node == nil {
		return nil
	} else {
		return ne.node.FileInfo()
	}
}

func (ne *nodeEvent) checkWritableEvent() error {
	if ne.Op&Remove == Remove || ne.Op&Chmod == Chmod {
		return errors.New("[nodeEvent/checkWritableevent] error: event type is not writable.")
	}

	ok, err := ne.IsDir()
	if err != nil {
		return err
	}
	if ok {
		p, err := ne.Path()
		if err != nil {
			return err
		}

		return errors.New(fmt.Sprintf("[nodeEvent/checkWritableevent] error: %s is directory.", p))
	}

	return nil
}

func (nes *nodeEvents) add(q eventQueue, eq *eventQueues, r *Root) error {
	ne := nodeEvent{
		Op: q.Op,
	}
	var node *Node
	var err error

	switch true {
	case q.Op&Create == Create:
		fi, err := fileinfo.Stat(q.Path())
		if err != nil {
			return err
		}

		if node = r.InoFind(fi.Ino()); node != nil {
			// when same inode found

			// rename dir of eventQueues
			eq.rename(node.Path(), fi.Path())

			// rename nodes
			if err = r.renameNode(node, fi.Dir(), fi.Name()); err != nil {
				return err
			}
		} else {
			// add root
			node, err = r.createAddNode(fi.Path())
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
			eq.addFromNodes(node.children())
		}

		ne.node = node
	case q.Op&Remove == Remove:
		if q.node == nil {
			return errors.New("[events/Add] Remove error: Node is nil.")
		}

		// set remove node info
		ne.node = q.node
		// remove node
		if q.Path() == q.node.Path() {
			r.removeNode(q.node)
		}
	case q.Op&Rename == Rename:
		if q.node == nil {
			return errors.New("[events/Add] Rename error: Node is nil.")
		}

		// set rename node info
		ne.node = q.node

		// rename finished on Create event.
		// delete current node on thie condition.

		// when fail rename
		if q.Path() == q.node.Path() {
			// don't happen rename function
			r.removeNode(q.node)
		}
	case q.Op&Write == Write:
		// TODO add node when nonexist
		if q.node != nil {
			ne.node = q.node
			r.appendWriteNodes(ne)
		}

		return nil
	case q.Op&Chmod == Chmod:
		return nil
	}

	ino, err := ne.Ino()
	if err != nil {
		return err
	}

	// find same inode event.
	targetI := -1
	for index, e := range *nes {
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
		*nes = append(*nes, ne)
		return nil
	}

	// merge same inode event. (pattern is Move only.)
	targetEvent := (*nes)[targetI]

	// Move Pattern: Create + Remove or Create + Rename
	switch true {
	case targetEvent.Op&Create == Create:
		switch true {
		case ne.Op&Remove == Remove, ne.Op&Rename == Rename:
			ne.beforePath = q.Path()
			if err != nil {
				return err
			}

			ne.node = targetEvent.node
		}

		ne.Op |= targetEvent.Op
	case targetEvent.Op&Remove == Remove, targetEvent.Op&Rename == Rename:
		switch true {
		case ne.Op&Create == Create:
			ne.beforePath, err = targetEvent.Path()
			if err != nil {
				return err
			}
		}

		ne.Op |= targetEvent.Op
	}

	(*nes)[targetI] = ne

	return nil
}

func (nes *nodeEvents) updateOp() {
	for i, ne := range *nes {
		if ne.Op&Create == Create && ne.Op&(Remove|Rename) > 0 {
			ne.Op = Move
			(*nes)[i] = ne
		}
	}
}
