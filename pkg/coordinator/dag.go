package coordinator

import (
	"fmt"
	"maps"

	"github.com/dominikbraun/graph"
)

type DAG struct {
	Graph      graph.Graph[string, GraphNode]
	Levels     [][]string // grouped by parallel level
	TaskLevels [][]Task
	NodeMap    map[string]GraphNode
	Edges      [][2]string // [from, to] pairs
	MaxLevel   int
}

func NewDAG() *DAG {
	g := graph.New(func(n GraphNode) string {
		return n.ID
	}, graph.Directed(), graph.PreventCycles())

	return &DAG{
		Graph: g,
	}
}

func (d *DAG) InspectGraph() {
	fmt.Println("\n🔍 Graph inspection:")

	// Topological order
	order, _ := graph.TopologicalSort(d.Graph)
	fmt.Printf("   Topological order: %v\n", order)

	// Predecessors — who depends on who
	predMap, _ := d.Graph.PredecessorMap()
	fmt.Println("   Dependencies:")
	for node, preds := range predMap {
		if len(preds) == 0 {
			fmt.Printf("     %s ← (root, no deps)\n", node)
		} else {
			deps := []string{}
			for dep := range preds {
				deps = append(deps, dep)
			}
			fmt.Printf("     %s ← %v\n", node, deps)
		}
	}

	// Adjacency — who feeds into who
	adjMap, _ := d.Graph.AdjacencyMap()
	fmt.Println("   Feeds into:")
	for node, targets := range adjMap {
		if len(targets) == 0 {
			fmt.Printf("     %s → (terminal)\n", node)
		} else {
			next := []string{}
			for t := range targets {
				next = append(next, t)
			}
			fmt.Printf("     %s → %v\n", node, next)
		}
	}

	// Levels
	fmt.Println("   Parallel levels:")
	for i, level := range d.Levels {
		parallel := ""
		if len(level) > 1 {
			parallel = " ⚡ PARALLEL"
		}
		fmt.Printf("     L%d: %v%s\n", i, level, parallel)
	}

	// Transitive reduction — minimal edges needed
	reduced, err := graph.TransitiveReduction(d.Graph)
	if err == nil {
		redAdj, _ := reduced.AdjacencyMap()
		edgeCount := 0
		for _, targets := range redAdj {
			edgeCount += len(targets)
		}
		fmt.Printf("   Transitive reduction: %d → %d edges (minimal needed)\n",
			len(d.Edges), edgeCount)
	}
}

func (d *DAG) CompileTasks(tasks []Task) (*DAG, error) {
	nodeMap := map[string]GraphNode{}
	var edges [][2]string
	var lastNodeIDs []string // tracks "who finished last" for sequential wiring

	for _, task := range tasks {
		process, err := d.knitGraph(task, lastNodeIDs)
		if err != nil {
			return nil, err
		}

		lastNodeIDs = process.lastNodeIDs
		edges = append(edges, process.edges...)

		maps.Copy(nodeMap, process.node)
	}

	_, err := graph.TopologicalSort(d.Graph)
	if err != nil {
		return nil, fmt.Errorf("cycle detected: %w", err)
	}

	levels := GraphNode{}.OrderingLevel(nodeMap, edges)

	taskLevels := make([][]Task, len(levels))

	maxLevel := 0
	for level, nodeIDs := range levels {
		for _, id := range nodeIDs {
			n := nodeMap[id]
			n.Level = level
			nodeMap[id] = n

			taskLevels[level] = append(taskLevels[level], nodeMap[id].Task)
			// Re-add vertex with updated level
		}
		if level > maxLevel {
			maxLevel = level
		}
	}

	return &DAG{
		Levels:     levels,
		Graph:      d.Graph,
		NodeMap:    nodeMap,
		TaskLevels: taskLevels,
		Edges:      edges,
		MaxLevel:   maxLevel,
	}, nil
}

func (d *DAG) knitGraph(task Task, lastNodeIDs []string) (knitProcess, error) {
	switch task.Type {
	case Fork:
		return task.processFork(d, lastNodeIDs)
	case Join:
		return task.processJoin(d, lastNodeIDs)
	case DynamicFork:
		return task.processDynamicFork(d, lastNodeIDs)
	default:
		return task.processDefault(d, lastNodeIDs)
	}
}
