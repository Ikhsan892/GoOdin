package coordinator

import "time"

type GraphNode struct {
	ID         string
	Name       string
	Mode       string
	Level      int
	Compensate string
	DependsOn  []string
	Status     string // pending | running | completed | failed | skipped | compensated
	Duration   time.Duration
}

func (c GraphNode) OrderingLevel(nodes map[string]GraphNode, edges [][2]string) [][]string {
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
