package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type axTree struct {
	nodeMap    map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode
	rootIDs    []proto.AccessibilityAXNodeID
	totalNodes int
}

type refTarget struct {
	Ref           string
	BackendNodeID proto.DOMBackendNodeID
	Disabled      bool
}

type refIndex struct {
	ordered  []refTarget
	byRef    map[string]refTarget
	byNodeID map[proto.AccessibilityAXNodeID]refTarget
}

func loadAXTree(page *rod.Page) (*axTree, error) {
	result, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("get accessibility tree: %w", err)
	}

	tree := &axTree{
		nodeMap:    make(map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode, len(result.Nodes)),
		totalNodes: len(result.Nodes),
	}
	for _, node := range result.Nodes {
		tree.nodeMap[node.NodeID] = node
		if node.ParentID == "" {
			tree.rootIDs = append(tree.rootIDs, node.NodeID)
		}
	}
	return tree, nil
}

func buildRefIndex(tree *axTree) *refIndex {
	index := &refIndex{
		ordered:  []refTarget{},
		byRef:    map[string]refTarget{},
		byNodeID: map[proto.AccessibilityAXNodeID]refTarget{},
	}

	refCounter := 0
	for _, rootID := range tree.rootIDs {
		walkInteractiveNodes(tree.nodeMap, rootID, index, &refCounter)
	}

	return index
}

func buildPageRefIndex(page *rod.Page) (*refIndex, error) {
	tree, err := loadAXTree(page)
	if err != nil {
		return nil, err
	}
	return buildRefIndex(tree), nil
}

func walkInteractiveNodes(
	nodeMap map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode,
	nodeID proto.AccessibilityAXNodeID,
	index *refIndex,
	refCounter *int,
) {
	axNode, ok := nodeMap[nodeID]
	if !ok {
		return
	}

	if axNode.Ignored {
		for _, childID := range axNode.ChildIDs {
			walkInteractiveNodes(nodeMap, childID, index, refCounter)
		}
		return
	}

	role := axValueStr(axNode.Role)
	if interactiveRoles[role] {
		*refCounter++
		target := refTarget{
			Ref:           fmt.Sprintf("@%d", *refCounter),
			BackendNodeID: axNode.BackendDOMNodeID,
			Disabled:      axNodeDisabled(axNode),
		}
		index.ordered = append(index.ordered, target)
		index.byRef[target.Ref] = target
		index.byNodeID[nodeID] = target
	}

	for _, childID := range axNode.ChildIDs {
		walkInteractiveNodes(nodeMap, childID, index, refCounter)
	}
}

func axNodeDisabled(axNode *proto.AccessibilityAXNode) bool {
	for _, prop := range axNode.Properties {
		if prop.Name == proto.AccessibilityAXPropertyNameDisabled && prop.Value != nil {
			return axValueStr(prop.Value) == "true"
		}
	}
	return false
}

func (idx *refIndex) count() int {
	return len(idx.ordered)
}

func (idx *refIndex) targetForRef(ref string) (refTarget, bool) {
	target, ok := idx.byRef[ref]
	return target, ok
}

func (idx *refIndex) targetForNode(nodeID proto.AccessibilityAXNodeID) (refTarget, bool) {
	target, ok := idx.byNodeID[nodeID]
	return target, ok
}

func normalizeRef(ref string) (string, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(ref, "@"))
	idx, err := strconv.Atoi(raw)
	if err != nil || idx < 1 {
		return "", fmt.Errorf("invalid ref %q: must be @N where N >= 1", ref)
	}
	return fmt.Sprintf("@%d", idx), nil
}
