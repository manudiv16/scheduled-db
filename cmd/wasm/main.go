package main

import (
	"encoding/json"
	"syscall/js"
	"time"

	"scheduled-db/internal/simulator"
	"scheduled-db/internal/store"
)

var sim *simulator.Simulator

func main() {
	cfg := simulator.DefaultConfig()
	cfg.SlotGap = 10 * time.Second
	cfg.SuccessRate = 0.95
	cfg.LatencyMs = 100

	sim = simulator.NewSimulator(cfg)

	exports := js.Global().Get("Object").New()

	exports.Set("createJob", js.FuncOf(createJob))
	exports.Set("deleteJob", js.FuncOf(deleteJob))
	exports.Set("tick", js.FuncOf(tick))
	exports.Set("advanceTime", js.FuncOf(advanceTime))
	exports.Set("getState", js.FuncOf(getState))
	exports.Set("getStateJSON", js.FuncOf(getStateJSON))
	exports.Set("setClockSpeed", js.FuncOf(setClockSpeed))
	exports.Set("getClockSpeed", js.FuncOf(getClockSpeed))
	exports.Set("setSuccessRate", js.FuncOf(setSuccessRate))
	exports.Set("setTime", js.FuncOf(setTime))
	exports.Set("getNow", js.FuncOf(getNow))
	exports.Set("reset", js.FuncOf(reset))

	js.Global().Set("simulator", exports)

	select {}
}

func createJob(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResult("expected JSON string argument")
	}

	var req store.CreateJobRequest
	if err := json.Unmarshal([]byte(args[0].String()), &req); err != nil {
		return errorResult("invalid JSON: " + err.Error())
	}

	job, err := sim.CreateJob(req)
	if err != nil {
		return errorResult(err.Error())
	}

	data, _ := json.Marshal(job)
	return successResult(string(data))
}

func deleteJob(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResult("expected job ID argument")
	}

	if err := sim.DeleteJob(args[0].String()); err != nil {
		return errorResult(err.Error())
	}

	return successResult("null")
}

func tick(this js.Value, args []js.Value) interface{} {
	result := sim.Tick()

	data, _ := json.Marshal(map[string]interface{}{
		"executed":      len(result.ExecutedJobs),
		"rescheduled":   len(result.Rescheduled),
		"ready_slots":   result.ReadySlots,
		"executed_jobs": result.ExecutedJobs,
		"rescheduled_ids": result.Rescheduled,
	})
	return successResult(string(data))
}

func advanceTime(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResult("expected seconds argument")
	}

	seconds := args[0].Float()
	now := sim.AdvanceTime(seconds)

	data, _ := json.Marshal(map[string]interface{}{
		"now": now,
	})
	return successResult(string(data))
}

func getState(this js.Value, args []js.Value) interface{} {
	state := sim.GetState()
	data, _ := json.Marshal(state)
	return string(data)
}

func getStateJSON(this js.Value, args []js.Value) interface{} {
	return sim.GetStateJSON()
}

func setClockSpeed(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResult("expected speed argument")
	}
	sim.SetClockSpeed(args[0].Float())
	return successResult("null")
}

func getClockSpeed(this js.Value, args []js.Value) interface{} {
	return sim.GetClockSpeed()
}

func setSuccessRate(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResult("expected rate argument")
	}
	sim.SetSuccessRate(args[0].Float())
	return successResult("null")
}

func setTime(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorResult("expected timestamp argument")
	}
	sim.SetTime(int64(args[0].Float()))
	return successResult("null")
}

func getNow(this js.Value, args []js.Value) interface{} {
	return sim.GetNow()
}

func reset(this js.Value, args []js.Value) interface{} {
	sim.Reset()
	return successResult("null")
}

func successResult(data string) map[string]interface{} {
	return map[string]interface{}{
		"ok":   true,
		"data": data,
	}
}

func errorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"ok":    false,
		"error": msg,
	}
}
