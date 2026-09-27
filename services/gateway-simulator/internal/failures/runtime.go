package failures

import (
	"sync"
	"time"
)

type Runtime struct {
	mutex       sync.RWMutex
	engine      *Engine
	active      map[Kind]map[string]struct{}
	pending     map[Kind]bool
	expires     time.Time
	awaitLaunch bool
}

func NewRuntime(engine *Engine) *Runtime {
	return &Runtime{engine: engine, active: make(map[Kind]map[string]struct{}), pending: make(map[Kind]bool)}
}

func NewLiveRuntime(engine *Engine) *Runtime {
	runtime := NewRuntime(engine)
	runtime.awaitLaunch = true
	return runtime
}

func (runtime *Runtime) Advance(now time.Time) {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	runtime.advance(now, "")
}

func (runtime *Runtime) TargetCommand(now time.Time, eventID, deviceID string) map[Kind]bool {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	runtime.advance(now, "")
	return runtime.targetCommand(eventID, deviceID)
}

func (runtime *Runtime) RecordCommand(now time.Time, eventID, deviceID string, setpointKW float64) map[Kind]bool {
	runtime.mutex.Lock()
	defer runtime.mutex.Unlock()
	if runtime.awaitLaunch && eventID != "" && setpointKW != 0 {
		shift := now.Sub(runtime.engine.scenario.Start)
		runtime.engine.scenario.Start = now
		for index := range runtime.engine.scenario.Injections {
			runtime.engine.scenario.Injections[index].At = runtime.engine.scenario.Injections[index].At.Add(shift)
		}
		runtime.awaitLaunch = false
	}
	runtime.engine.recordCommand(eventID, deviceID, setpointKW)
	runtime.advance(now, eventID)
	return runtime.targetCommand(eventID, deviceID)
}

func (runtime *Runtime) targetCommand(eventID, deviceID string) map[Kind]bool {
	if eventID == "" || deviceID == "" {
		return nil
	}
	var targeted map[Kind]bool
	for kind := range runtime.pending {
		if targeted == nil {
			targeted = make(map[Kind]bool, len(runtime.pending))
		}
		targeted[kind] = true
		delete(runtime.pending, kind)
	}
	return targeted
}

func (runtime *Runtime) advance(now time.Time, eventID string) {
	if runtime.awaitLaunch {
		return
	}
	if !runtime.expires.IsZero() && !now.Before(runtime.expires) {
		clear(runtime.active)
		runtime.expires = time.Time{}
	}
	for _, effect := range runtime.engine.advance(now, eventID) {
		if effect.Scope == NextCommand {
			runtime.pending[effect.Kind] = true
			continue
		}
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
