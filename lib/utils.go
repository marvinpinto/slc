package lib

import (
	"encoding/json"
	"fmt"
)

type ProgressBar interface {
	SetTotal(total int64, triggerComplete bool)
	Increment()
	Current() int64
}

type StubProgressBar struct{}

func (s *StubProgressBar) SetTotal(a int64, b bool) {}
func (s *StubProgressBar) Increment()               {}
func (s *StubProgressBar) Current() int64           { return int64(0) }

func debugObject(inp interface{}) string {
	o, err := json.Marshal(inp)
	if err != nil {
		return fmt.Sprintf("%v", inp)
	}
	return string(o)
}
