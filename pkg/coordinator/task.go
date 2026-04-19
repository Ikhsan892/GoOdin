package coordinator

import "fmt"

type TaskType string

const (
	ForkJoin TaskType = "FORK_JOIN"
	Simple   TaskType = "SIMPLE"
	Switch   TaskType = "SWITCH"
)

type Task struct {
	Name       string   `yaml:"name"`
	Ref        string   `yaml:"ref"`
	Type       TaskType `yaml:"type"` // SIMPLE | FORK_JOIN | SWITCH
	Mode       string   `yaml:"mode"` // sync | async | outbox
	Compensate string   `yaml:"compensate"`
	ForkTasks  [][]Task `yaml:"fork_tasks"` // for FORK_JOIN
	JoinOn     []string `yaml:"join_on"`
	DependsOn  []string `yaml:"depends_on"` // explicit override
}

type knitProcess struct {
	node        map[string]GraphNode
	edges       [][2]string
	lastNodeIDs []string
}

func (t Task) processDefault(d *DAG, lastNodeIDs []string) (knitProcess, error) {
	var branchEndIDs []string
	var edges [][2]string

	nodeMap := make(map[string]GraphNode)

	node := GraphNode{
		ID:         t.Ref,
		Name:       t.Name,
		Mode:       t.Mode,
		Compensate: t.Compensate,
	}

	if err := d.Graph.AddVertex(node); err != nil {
		return knitProcess{}, fmt.Errorf("add vertex %s: %w", node.ID, err)
	}

	// Wire dependencies
	deps := t.DependsOn
	if len(deps) == 0 {
		deps = lastNodeIDs // implicit: depends on previous
	}
	for _, dep := range deps {
		if err := d.Graph.AddEdge(dep, node.ID); err != nil {
			return knitProcess{}, fmt.Errorf("add edge %s→%s: %w", dep, node.ID, err)
		}
		edges = append(edges, [2]string{dep, node.ID})
		node.DependsOn = append(node.DependsOn, dep)
	}

	nodeMap[node.ID] = node
	branchEndIDs = []string{node.ID}

	return knitProcess{
		node:        nodeMap,
		edges:       edges,
		lastNodeIDs: branchEndIDs,
	}, nil
}

func (t Task) processForkJoin(d *DAG, lastNodeIDs []string) (knitProcess, error) {
	var branchEndIDs []string
	var edges [][2]string

	nodeMap := make(map[string]GraphNode)

	for _, branch := range t.ForkTasks {
		var prevInBranch string

		for _, bt := range branch {
			node := GraphNode{
				ID:         bt.Ref,
				Name:       bt.Name,
				Mode:       bt.Mode,
				Compensate: bt.Compensate,
			}

			// Add vertex to graph
			if err := d.Graph.AddVertex(node); err != nil {
				return knitProcess{}, fmt.Errorf("add vertex %s: %w", node.ID, err)
			}

			nodeMap[node.ID] = node

			if prevInBranch != "" {
				// Sequential WITHIN branch
				d.Graph.AddEdge(prevInBranch, node.ID)
				edges = append(edges, [2]string{prevInBranch, node.ID})
				node.DependsOn = append(node.DependsOn, prevInBranch)
			} else {
				// First in branch → depends on whatever came before FORK
				for _, dep := range lastNodeIDs {
					d.Graph.AddEdge(dep, node.ID)
					edges = append(edges, [2]string{dep, node.ID})
					node.DependsOn = append(node.DependsOn, dep)
				}
			}

			nodeMap[node.ID] = node
			prevInBranch = node.ID
		}

		branchEndIDs = append(branchEndIDs, prevInBranch)
	}

	return knitProcess{
		lastNodeIDs: branchEndIDs,
		edges:       edges,
		node:        nodeMap,
	}, nil
}
