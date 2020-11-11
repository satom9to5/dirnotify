package dirnotify

import (
	"errors"
)

type NodeMap map[uint64]*Node

func (nm *NodeMap) get(ino uint64) *Node {
	if n, ok := (*nm)[ino]; ok {
		return n
	} else {
		return nil
	}
}

func (nm *NodeMap) add(n *Node) error {
	ino := n.Ino()
	if ino == 0 {
		return errors.New("[NodeMap/add] error: inode is empty.")
	}

	(*nm)[ino] = n

	return nil
}

func (nm *NodeMap) remove(ino uint64) error {
	if _, ok := (*nm)[ino]; ok {
		delete(*nm, ino)

		return nil
	} else {
		return errors.New("[NodeMap/remove] error: cannot find node.")
	}
}

func (nm *NodeMap) checkWriteComplete() []*Node {
	nodes := []*Node{}

	for ino, node := range *nm {
		// get current value before update
		preTime := node.ModTime()
		preSize := node.Size()

		if err := node.Stat(); err == nil {
			if node.ModTime() == preTime && node.Size() == preSize {
				nodes = append(nodes, node)

				nm.remove(ino)
			}
		} else {
			// when file removed.
			nm.remove(ino)
		}
	}

	return nodes
}
