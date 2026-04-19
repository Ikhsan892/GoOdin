package coordinator

import (
	"fmt"
	"maps"

	"github.com/dominikbraun/graph"
)

type DAG struct {
	Graph    graph.Graph[string, GraphNode]
	Levels   [][]string // grouped by parallel level
	NodeMap  map[string]GraphNode
	Edges    [][2]string // [from, to] pairs
	MaxLevel int
}

func NewDAG() *DAG {
	g := graph.New(func(n GraphNode) string {
		return n.ID
	}, graph.Directed(), graph.PreventCycles())

	return &DAG{
		Graph: g,
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

	maxLevel := 0
	for level, nodeIDs := range levels {
		for _, id := range nodeIDs {
			n := nodeMap[id]
			n.Level = level
			nodeMap[id] = n
			// Re-add vertex with updated level
		}
		if level > maxLevel {
			maxLevel = level
		}
	}

	return &DAG{
		Levels:   levels,
		Graph:    d.Graph,
		NodeMap:  nodeMap,
		Edges:    edges,
		MaxLevel: maxLevel,
	}, nil
}

// assignLevels — BFS level assignment
// Nodes with no dependencies = level 0
// Others = max(dependency levels) + 1
func assignLevels(nodes map[string]GraphNode, edges [][2]string) [][]string {
	// Build adjacency
	inDeps := map[string][]string{} // node → who it depends on
	for _, e := range edges {
		inDeps[e[1]] = append(inDeps[e[1]], e[0])
	}

	levels := map[string]int{}

	// Iteratively assign levels
	changed := true
	for changed {
		changed = false
		for id := range nodes {
			if _, ok := levels[id]; ok {
				continue
			}

			deps := inDeps[id]
			if len(deps) == 0 {
				levels[id] = 0
				changed = true
				continue
			}

			// All deps must have levels assigned
			allResolved := true
			maxDepLevel := 0
			for _, dep := range deps {
				depLevel, ok := levels[dep]
				if !ok {
					allResolved = false
					break
				}
				if depLevel > maxDepLevel {
					maxDepLevel = depLevel
				}
			}

			if allResolved {
				levels[id] = maxDepLevel + 1
				changed = true
			}
		}
	}

	// Group by level
	maxLevel := 0
	for _, l := range levels {
		if l > maxLevel {
			maxLevel = l
		}
	}

	grouped := make([][]string, maxLevel+1)
	for id, l := range levels {
		grouped[l] = append(grouped[l], id)
	}

	return grouped
}

func (d *DAG) knitGraph(task Task, lastNodeIDs []string) (knitProcess, error) {
	switch task.Type {
	case ForkJoin:
		return task.processForkJoin(d, lastNodeIDs)
	default:
		return task.processDefault(d, lastNodeIDs)
	}
}
