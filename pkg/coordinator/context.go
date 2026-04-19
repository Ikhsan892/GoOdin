package coordinator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// ════════════════════════════════════════════════════════════
// CONTEXT STORE — in-memory, per execution
// ════════════════════════════════════════════════════════════

type ContextStore struct {
	mu   sync.RWMutex
	data map[string]any // flat namespace: "validate" → {output map}
}

func NewContextStore() *ContextStore {
	return &ContextStore{
		data: map[string]any{},
	}
}

// Set — store step result by namespace
//
//	ctx.Set("validate", map[string]any{"is_valid": true, "clean_items": [...]})
//	ctx.Set("pricing", map[string]any{"final_total": 99000, "discount": 10})
func (c *ContextStore) Set(namespace string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[namespace] = value
}

// Get — resolve dot-path like "pricing.final_total" or "routing.groups[0].station"
func (c *ContextStore) Get(path string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	t := template.Must(
		template.New("base").
			Funcs(sprig.FuncMap()).
			Parse(path),
	)

	var buf bytes.Buffer
	err := t.Execute(&buf, c.data)
	if err != nil {
		return nil, err
	}

	return buf.String(), nil
}

// Snapshot — full context (for debug/logging)
func (c *ContextStore) Snapshot() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make(map[string]any, len(c.data))
	for k, v := range c.data {
		cp[k] = v
	}
	return cp
}

// ════════════════════════════════════════════════════════════
// PATH RESOLVER — "pricing.final_total", "routing.groups[0].station"
// ════════════════════════════════════════════════════════════

// resolvePath walks nested map/slice by dot-separated path
// Supports:
//
//	"pricing.final_total"          → simple nested field
//	"routing.groups[0].station"    → array index
//	"routing.groups[*].station"    → wildcard = collect all
//	"input.order.items[0].name"    → deep nesting
func resolvePath(data map[string]any, path string) (any, error) {
	parts := splitPath(path)
	var current any = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part.Key]
			if !ok {
				return nil, fmt.Errorf("key %q not found in path %q", part.Key, path)
			}
			if part.Index >= 0 {
				arr, ok := val.([]any)
				if !ok {
					return nil, fmt.Errorf("expected array at %q in path %q", part.Key, path)
				}
				if part.Index >= len(arr) {
					return nil, fmt.Errorf("index %d out of range at %q", part.Index, part.Key)
				}
				current = arr[part.Index]
			} else if part.Wildcard {
				arr, ok := val.([]any)
				if !ok {
					return nil, fmt.Errorf("expected array at %q for wildcard", part.Key)
				}
				current = arr // pass full array, next parts apply per-element
			} else {
				current = val
			}
		default:
			return nil, fmt.Errorf("cannot traverse %T at %q", current, part.Key)
		}
	}
	return current, nil
}

type pathPart struct {
	Key      string
	Index    int  // -1 = no index
	Wildcard bool // [*]
}

var arrayPattern = regexp.MustCompile(`^(\w+)\[(\d+|\*)]$`)

func splitPath(path string) []pathPart {
	segments := strings.Split(path, ".")
	parts := make([]pathPart, 0, len(segments))

	for _, seg := range segments {
		if m := arrayPattern.FindStringSubmatch(seg); m != nil {
			key := m[1]
			if m[2] == "*" {
				parts = append(parts, pathPart{Key: key, Index: -1, Wildcard: true})
			} else {
				idx, _ := strconv.Atoi(m[2])
				parts = append(parts, pathPart{Key: key, Index: idx})
			}
		} else {
			parts = append(parts, pathPart{Key: seg, Index: -1})
		}
	}
	return parts
}

// ════════════════════════════════════════════════════════════
// TEMPLATE RESOLVER — resolve {step.output.field} in input mappings
// ════════════════════════════════════════════════════════════

// Pattern: {validate.clean_items} or {pricing.final_total}
var tmplPattern = regexp.MustCompile(`\{([^}]+)}`)

// ResolveTemplate resolves all {path} references in a value against context
//
// Input mapping from YAML:
//
//	input_params:
//	  order_id: "{input.order.id}"
//	  amount: "{pricing.final_total}"
//	  items: "{routing.groups}"
//	  station: "{routing.groups[0].station}"
//
// Returns resolved map with actual values from context
func (c *ContextStore) ResolveTemplate(params map[string]any) (map[string]any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resolved := make(map[string]any, len(params))

	for key, val := range params {
		switch v := val.(type) {
		case string:
			resolved[key] = c.resolveStringValue(v)

		case map[string]any:
			// Nested map — recurse
			inner, err := c.resolveMapValue(v)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", key, err)
			}
			resolved[key] = inner

		default:
			// Literal value (int, bool, etc) — pass through
			resolved[key] = val
		}
	}

	return resolved, nil
}

// resolveStringValue handles 3 cases:
//
//  1. Pure reference: "{pricing.final_total}" → returns actual type (int, map, slice)
//  2. Mixed template: "Order {input.order.id} total {pricing.final_total}" → string interpolation
//  3. No template: "hello" → pass through
func (c *ContextStore) resolveStringValue(s string) any {
	str, err := c.Get(s)
	if err != nil {
		return ""
	}

	return str.(string)
}

func (c *ContextStore) resolveMapValue(m map[string]any) (map[string]any, error) {
	resolved := make(map[string]any, len(m))
	for k, v := range m {
		switch vt := v.(type) {
		case string:
			resolved[k] = c.resolveStringValue(vt)
		case map[string]any:
			inner, err := c.resolveMapValue(vt)
			if err != nil {
				return nil, err
			}
			resolved[k] = inner
		default:
			resolved[k] = v
		}
	}
	return resolved, nil
}

// ════════════════════════════════════════════════════════════
// JQ TRANSFORM — for complex reshaping
// ════════════════════════════════════════════════════════════

// JQ runs a jq expression against context data
// Useful for complex transforms that are ugly in templates:
//
// YAML usage:
//
//	transform:
//	  jq: ".routing.groups | map({station: .station, count: (.items | length)})"
//
// Or per-field:
//
//	input_params:
//	  stations:
//	    jq: "[.routing.groups[].station]"
//	  total_items:
//	    jq: "[.routing.groups[].items[]] | length"
func (c *ContextStore) JQ(expression string) (any, error) {
	c.mu.RLock()
	snapshot := c.data
	c.mu.RUnlock()

	// Serialize context to JSON
	inputJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal context: %w", err)
	}

	// Shell out to jq binary
	cmd := exec.Command("jq", "-c", expression)
	cmd.Stdin = strings.NewReader(string(inputJSON))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("jq %q: %w", expression, err)
	}

	// Parse result back
	var result any
	if err := json.Unmarshal(output, &result); err != nil {
		// jq returned raw string
		return strings.TrimSpace(string(output)), nil
	}

	return result, nil
}

// JQInline — pure Go jq alternative for simple expressions
// No external binary needed. Covers 80% of cases.
// For complex stuff, fall back to JQ() which shells out.
func (c *ContextStore) JQInline(expression string) (any, error) {
	switch {
	case strings.HasPrefix(expression, "."):
		// Simple path: ".pricing.final_total" → same as Get
		path := strings.TrimPrefix(expression, ".")
		return c.Get(path)
	default:
		// Complex — shell out
		return c.JQ(expression)
	}
}
