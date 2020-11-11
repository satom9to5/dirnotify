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
