package valueobject

import "errors"

// Points is an immutable value object representing loyalty points.
// Always use NewPoints to construct — never assign the zero value directly.
type Points struct {
	value int
}

var ErrNegativePoints = errors.New("points cannot be negative")

// NewPoints constructs a Points value object. Returns an error for negative values.
func NewPoints(v int) (Points, error) {
	if v < 0 {
		return Points{}, ErrNegativePoints
	}
	return Points{value: v}, nil
}

// MustNewPoints constructs Points and panics on invalid input. Use only in tests or constants.
func MustNewPoints(v int) Points {
	p, err := NewPoints(v)
	if err != nil {
		panic(err)
	}
	return p
}

func (p Points) Value() int { return p.value }

func (p Points) Add(other Points) Points {
	return Points{value: p.value + other.value}
}

func (p Points) Sub(other Points) (Points, error) {
	if p.value < other.value {
		return Points{}, errors.New("insufficient points")
	}
	return Points{value: p.value - other.value}, nil
}

func (p Points) IsZero() bool { return p.value == 0 }
