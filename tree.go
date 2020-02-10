package dtree

import (
	"context"
	"encoding/json"
	"sort"
)

// Operator represent a function that will evaluate a node
type Operator func(requests map[string]interface{}, node *Tree) (*Tree, error)

// TreeOptions allow to extend the comparator
type TreeOptions struct {
	StopIfConvertingError    bool
	Operators                map[string]Operator
	OverrideExistingOperator bool
	Context                  context.Context
}

// Tree represent a Tree
type Tree struct {
	nodes  []*Tree
	parent *Tree

	ctx context.Context

	ID       int         `json:"id"`
	Name     string      `json:"name"`
	ParentID int         `json:"parent_id"`
	Value    interface{} `json:"value"`
	Operator string      `json:"operator"`
	Key      string      `json:"key"`
	Order    int         `json:"order"`
	Content  interface{} `json:"content"`
}

type byOrder []*Tree

func (o byOrder) Len() int      { return len(o) }
func (o byOrder) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o byOrder) Less(i, j int) bool {
	// fallback is always the last
	if s, ok := o[i].Value.(string); ok {
		if s == FallbackType {
			return false
		}
	}

	if s, ok := o[j].Value.(string); ok {
		if s == FallbackType {
			return true
		}
	}

	return o[i].Order < o[j].Order
}

// AddNode Add a new Node (leaf) to the Tree
func (t *Tree) AddNode(node *Tree) {
	node.parent = t
	t.nodes = append(t.nodes, node)
	sort.Sort(byOrder(t.nodes))
}

// GetChild get the nodes child of this one
func (t *Tree) GetChild() []*Tree {
	return t.nodes
}

// GetParent get the parent node of this one
func (t *Tree) GetParent() *Tree {
	return t.parent
}

// WithContext returns a tree with a context
func (t *Tree) WithContext(ctx context.Context) *Tree {
	t.ctx = ctx
	return t
}

// Context returns the context
func (t *Tree) Context() context.Context {
	return t.ctx
}

// Next evaluate which will be the next Node according to the jsonRequest
func (t *Tree) Next(jsonRequest map[string]interface{}, config *TreeOptions) (*Tree, error) {
	var jsonValue interface{}
	var oldName string
	for _, n := range t.nodes {

		if oldName != n.Key {
			jsonValue = jsonRequest[n.Key]
			oldName = n.Key
		}

		selected, err := compare(jsonRequest, jsonValue, n, config)
		if config.StopIfConvertingError == true && err != nil {
			return n, err
		}

		if selected != nil {
			if t.ctx != nil {
				t.ctx = contextValue(t.ctx, n.ID, n.Key, jsonValue, n.Operator, n.Value)
			}
			if config.Context != nil {
				config.Context = contextValue(config.Context, n.ID, n.Key, jsonValue, n.Operator, n.Value)
			}
			return selected, nil
		}
	}

	return nil, nil
}

// LoadTree gets a json on build the Tree related
func LoadTree(jsonTree []byte) (*Tree, error) {
	var trees []Tree
	err := json.Unmarshal(jsonTree, &trees)
	if err != nil {
		return nil, err
	}

	return CreateTree(trees), nil
}

// CreateTree attach the nodes to the Tree
func CreateTree(data []Tree) *Tree {
	temp := make(map[int]*Tree)
	var root *Tree
	for i := range data {
		leaf := &data[i]
		temp[leaf.ID] = leaf
		if leaf.ParentID == 0 {
			root = leaf
		}
	}

	for _, v := range temp {
		if v.ParentID != 0 {
			temp[v.ParentID].AddNode(v)
		}
	}

	return root
}

// ResolveJSON calculate which will be the selected node according to the jsonRequest
func (t *Tree) ResolveJSON(jsonRequest []byte, options ...func(t *TreeOptions)) (*Tree, error) {
	var request map[string]interface{}
	err := json.Unmarshal(jsonRequest, &request)
	if err != nil {
		return nil, err
	}

	return t.Resolve(request, options...)
}

// Resolve calculate which will be the selected node according to the map request
func (t *Tree) Resolve(request map[string]interface{}, options ...func(t *TreeOptions)) (*Tree, error) {
	config := &TreeOptions{}

	for _, option := range options {
		option(config)
	}

	if len(config.Operators) > 0 {
		for k := range config.Operators {
			if isExistingOperator(k) {
				config.OverrideExistingOperator = true
				break
			}
		}
	}

	return t.resolve(request, config)
}

func (t *Tree) resolve(request map[string]interface{}, config *TreeOptions) (*Tree, error) {
	temp, err := t.Next(request, config)
	if err != nil {
		return t, err
	}

	if temp == nil {
		return t, err
	}
	return temp.resolve(request, config)
}
