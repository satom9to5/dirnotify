package dirnotify

import (
	"errors"
	"log"
	// third party
	"github.com/satom9to5/fileinfo"
)

type nodeEvents []nodeEvent

func (nes *nodeEvents) add(eq eventQueue, eqs *eventQueues, r *Root) error {
	ne := nodeEvent{
		Op: eq.Op,
	}
	var node *Node
	var err error

	switch true {
	case eq.Op&Create == Create:
		fi, err := fileinfo.Stat(eq.Path())
		if err != nil {
			return err
		}

		if node = r.InoFind(fi.Ino()); node != nil {
			// when same inode found

			// rename dir of eventQueues
			eqs.rename(node.Path(), fi.Path())

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
			eqs.addFromNodes(node.children())
		}

		ne.node = node
	case eq.Op&Remove == Remove:
		if eq.node == nil {
			return errors.New("[events/add] Remove error: Node is nil.")
		}

		// set remove node info
		ne.node = eq.node
		// remove node
		if eq.Path() == eq.node.Path() {
			r.removeNode(eq.node)
		}
	case eq.Op&Rename == Rename:
		if eq.node == nil {
			return errors.New("[events/add] Rename error: Node is nil.")
		}

		// set rename node info
		ne.node = eq.node

		// rename finished on Create event.
		// delete current node on thie condition.

		// when fail rename
		if eq.Path() == eq.node.Path() {
			// don't happen rename function
			r.removeNode(eq.node)
		}
	case eq.Op&Write == Write:
		if eq.node != nil {
			ne.node = eq.node
			r.appendWriteNodes(ne)
			return nil
		}

		// add node when nonexist
		fi, err := fileinfo.Stat(eq.Path())
		if err != nil {
			return err
		}

		// add root
		ne.node, err = r.createAddNode(fi.Path())
		if err != nil {
			return err
		}

		// rewrite Event Type
		ne.Op = Create
	case eq.Op&Chmod == Chmod:
		return nil
	}

	// find same inode event.
	targetEvent, targetIndex, err := nes.findByFileInfo(ne.FileInfo())
	if err != nil {
		log.Println("[NodeEvent/add] not found fileInfo: " + ne.String())
		return err
	}

	if targetEvent == nil {
		*nes = append(*nes, ne)
		return nil
	}

	// merge same inode event. (pattern is Move only.)
	// Move Pattern: Create + Remove or Create + Rename
	switch true {
	case targetEvent.Op&Create == Create:
		switch true {
		case ne.Op&Remove == Remove, ne.Op&Rename == Rename:
			ne.beforePath = eq.Path()
			ne.node = targetEvent.node
			ne.Op |= targetEvent.Op
		}
	case targetEvent.Op&Remove == Remove, targetEvent.Op&Rename == Rename:
		switch true {
		case ne.Op&Create == Create:
			beforePath, err := targetEvent.Path()
			if err != nil {
				return err
			}
			ne.beforePath = beforePath
			ne.Op |= targetEvent.Op
		}
	}

	if ne.beforePath != "" {
		(*nes)[targetIndex] = ne
	}

	return nil
}

func (nes *nodeEvents) findByFileInfo(fi *fileinfo.FileInfo) (*nodeEvent, int, error) {
	if fi == nil {
		return nil, -1, nil
	}

	ino := fi.Ino()
	targetIndex := -1

	// find same inode event.
	for index, e := range *nes {
		eino, err := e.Ino()
		if err != nil {
			return nil, -1, err
		}

		if ino == eino {
			targetIndex = index
			break
		}
	}

	if targetIndex >= 0 {
		return &(*nes)[targetIndex], targetIndex, nil
	} else {
		return nil, -1, nil
	}
}

func (nes *nodeEvents) updateOp() {
	for i, ne := range *nes {
		if ne.Op&Create == Create && ne.Op&(Remove|Rename) > 0 {
			ne.Op = Move
			(*nes)[i] = ne
		}
	}
}
