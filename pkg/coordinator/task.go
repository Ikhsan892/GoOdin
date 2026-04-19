package coordinator

import "fmt"

type TaskType string

const (
	Fork         TaskType = "FORK"
	Join         TaskType = "JOIN"
	DynamicFork  TaskType = "DYNAMIC_FORK"
	Simple       TaskType = "SIMPLE"
	Switch       TaskType = "SWITCH"
	Inline       TaskType = "INLINE"
	Event        TaskType = "EVENT"
	HTTP         TaskType = "HTTP"
	Human        TaskType = "HUMAN"
	Noop         TaskType = "NOOP"
	JQ           TaskType = "JQ"
	FN           TaskType = "FN"
	KafkaPublish TaskType = "KAFKA_PUBLISH"
	Wait         TaskType = "WAIT"
	Terminate    TaskType = "TERMINATE"
)

type Task struct {
	Name       string   `yaml:"name"`
	Ref        string   `yaml:"ref"`
	Type       TaskType `yaml:"type"` // SIMPLE | FORK | JOIN | DYNAMIC_FORK | SWITCH | ...
	Mode       string   `yaml:"mode"` // sync | async | outbox
	Compensate string   `yaml:"compensate"`
	// Input mapping — templates resolved against context
	// "{pricing.final_total}" → actual value
	InputParams  map[string]any `json:"input_params"`
	ForkTasks    [][]Task       `yaml:"fork_tasks"`   // FORK / DYNAMIC_FORK: static parallel branches
	JoinOn       []string       `yaml:"join_on"`      // JOIN: refs to wait for (empty = all prior ends)
	DependsOn    []string       `yaml:"depends_on"`   // explicit override
	IsForkBranch bool           `yaml:"-"`            // set internally for tasks spawned inside a FORK
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
		Task:       t,
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

// processFork creates a FORK gateway node and wires parallel branches off it.
// The gateway itself is returned as lastNodeIDs so the main task flow continues
// without waiting for branches — only an explicit JOIN blocks until branches finish.
func (t Task) processFork(d *DAG, lastNodeIDs []string) (knitProcess, error) {
	var edges [][2]string
	nodeMap := make(map[string]GraphNode)

	// FORK gateway node — pass-through, fires branches then yields to main flow
	gateway := GraphNode{
		ID:         t.Ref,
		Name:       t.Name,
		Mode:       t.Mode,
		Compensate: t.Compensate,
		Task:       t,
	}
	if err := d.Graph.AddVertex(gateway); err != nil {
		return knitProcess{}, fmt.Errorf("add vertex %s: %w", gateway.ID, err)
	}
	for _, dep := range lastNodeIDs {
		if err := d.Graph.AddEdge(dep, gateway.ID); err != nil {
			return knitProcess{}, fmt.Errorf("add edge %s→%s: %w", dep, gateway.ID, err)
		}
		edges = append(edges, [2]string{dep, gateway.ID})
		gateway.DependsOn = append(gateway.DependsOn, dep)
	}
	nodeMap[gateway.ID] = gateway

	// Branch nodes — wired to gateway, run in parallel with main flow
	for _, branch := range t.ForkTasks {
		var prevInBranch string

		for _, bt := range branch {
			bt.IsForkBranch = true
			node := GraphNode{
				ID:         bt.Ref,
				Name:       bt.Name,
				Mode:       bt.Mode,
				Compensate: bt.Compensate,
				Task:       bt,
			}
			if err := d.Graph.AddVertex(node); err != nil {
				return knitProcess{}, fmt.Errorf("add vertex %s: %w", node.ID, err)
			}

			dep := gateway.ID
			if prevInBranch != "" {
				dep = prevInBranch
			}
			if err := d.Graph.AddEdge(dep, node.ID); err != nil {
				return knitProcess{}, fmt.Errorf("add edge %s→%s: %w", dep, node.ID, err)
			}
			edges = append(edges, [2]string{dep, node.ID})
			node.DependsOn = append(node.DependsOn, dep)

			nodeMap[node.ID] = node
			prevInBranch = node.ID
		}
	}

	// Return gateway ID — main flow continues from here, not from branch ends
	return knitProcess{
		lastNodeIDs: []string{gateway.ID},
		edges:       edges,
		node:        nodeMap,
	}, nil
}

func (t Task) processJoin(d *DAG, lastNodeIDs []string) (knitProcess, error) {
	node := GraphNode{
		ID:         t.Ref,
		Name:       t.Name,
		Mode:       t.Mode,
		Compensate: t.Compensate,
		Task:       t,
	}

	if err := d.Graph.AddVertex(node); err != nil {
		return knitProcess{}, fmt.Errorf("add vertex %s: %w", node.ID, err)
	}

	// Always wait for both: explicit branch refs (JoinOn) + last main-flow task (lastNodeIDs)
	seen := map[string]bool{}
	var deps []string
	for _, d := range append(t.JoinOn, lastNodeIDs...) {
		if !seen[d] {
			seen[d] = true
			deps = append(deps, d)
		}
	}

	var edges [][2]string
	for _, dep := range deps {
		// Skip refs that aren't graph vertices — dynamic fork branches exist only at runtime
		if _, err := d.Graph.Vertex(dep); err != nil {
			continue
		}
		if err := d.Graph.AddEdge(dep, node.ID); err != nil {
			return knitProcess{}, fmt.Errorf("add edge %s→%s: %w", dep, node.ID, err)
		}
		edges = append(edges, [2]string{dep, node.ID})
		node.DependsOn = append(node.DependsOn, dep)
	}

	return knitProcess{
		node:        map[string]GraphNode{node.ID: node},
		edges:       edges,
		lastNodeIDs: []string{node.ID},
	}, nil
}

func (t Task) processDynamicFork(d *DAG, lastNodeIDs []string) (knitProcess, error) {
	node := GraphNode{
		ID:         t.Ref,
		Name:       t.Name,
		Mode:       t.Mode,
		Compensate: t.Compensate,
		Task:       t,
	}

	if err := d.Graph.AddVertex(node); err != nil {
		return knitProcess{}, fmt.Errorf("add vertex %s: %w", node.ID, err)
	}

	var edges [][2]string
	for _, dep := range lastNodeIDs {
		if err := d.Graph.AddEdge(dep, node.ID); err != nil {
			return knitProcess{}, fmt.Errorf("add edge %s→%s: %w", dep, node.ID, err)
		}
		edges = append(edges, [2]string{dep, node.ID})
		node.DependsOn = append(node.DependsOn, dep)
	}

	return knitProcess{
		node:        map[string]GraphNode{node.ID: node},
		edges:       edges,
		lastNodeIDs: []string{node.ID},
	}, nil
}
