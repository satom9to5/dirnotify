package dirnotify

import (
	"errors"
	"fmt"
	"testing"
)

func testNodes(t *testing.T, node *Node) error {
	for _, file := range node.files {
		if file.IsDir() {
			return errors.New(fmt.Sprintf("node is not file: %s", node.Path()))
		}
	}

	for _, dir := range node.dirs {
		if !dir.IsDir() {
			return errors.New(fmt.Sprintf("node is not directory: %s", node.Path()))
		}
	}

	for _, dir := range node.dirs {
		if err := testNodes(t, dir); err != nil {
			return err
		}
	}

	return nil
}
