package failures

import (
	"sync"
	"time"
)

type Runtime struct {
	mutex   sync.RWMutex
	engine  *Engine
	active  map[Kind]map[string]struct{}
	expires time.Time
}

func NewRuntime(engine *Engine) *Runtime {
	return &Runtime{engine: engine, active: make(map[Kind]map[string]struct{})}
}

func (runtime *Runtime) Advance(now time.Time) {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	runtime.advance(now, "")
}

func (runtime *Runtime) RecordCommand(now time.Time, eventID, deviceID string) {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	runtime.engine.recordCommand(eventID, deviceID)
	runtime.advance(now, eventID)
}

func (runtime *Runtime) advance(now time.Time, eventID string) {
	if !runtime.expires.IsZero() && !now.Before(runtime.expires) {
		clear(runtime.active)
		runtime.expires = time.Time{}
	}
	for _, effect := range runtime.engine.advance(now, eventID) {
		if !now.Before(effect.At.Add(runtime.engine.scenario.Tick)) {
			continue
		}
		selected := make(map[string]struct{}, len(effect.DeviceIDs))
		for _, deviceID := range effect.DeviceIDs {
			selected[deviceID] = struct{}{}
		}
		runtime.active[effect.Kind] = selected
		runtime.expires = effect.At.Add(runtime.engine.scenario.Tick)
	}
}

func (runtime *Runtime) Affects(kind, deviceID string) bool {
	runtime.mutex.RLock()
	defer runtime.mutex.RUnlock()
	_, found := runtime.active[Kind(kind)][deviceID]
	return found
}
