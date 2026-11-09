package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/azure/azure-functions-golang-worker/middleware/durabletask"
	"github.com/azure/azure-functions-golang-worker/sdk"
	"github.com/azure/azure-functions-golang-worker/worker"
	"github.com/microsoft/durabletask-go/api"
	"github.com/microsoft/durabletask-go/task"
)

type GreetingRequest struct {
	Cities []string `json:"cities"`
}

func (input GreetingRequest) validate() error {
	if len(input.Cities) == 0 || len(input.Cities) > 32 {
		return errors.New("provide between 1 and 32 cities")
	}
	for _, city := range input.Cities {
		if strings.TrimSpace(city) == "" || len(city) > 80 {
			return errors.New("each city must contain between 1 and 80 bytes")
		}
	}
	return nil
}

func FanOutGreetings(ctx *task.OrchestrationContext) (any, error) {
	var input GreetingRequest
	if err := ctx.GetInput(&input); err != nil {
		return nil, err
	}
	if err := input.validate(); err != nil {
		return nil, err
	}

	ctx.SetCustomStatus("greeting cities")
	pending := make([]task.Task, len(input.Cities))
	// Schedule every activity before awaiting any result.
	for i, city := range input.Cities {
		pending[i] = ctx.CallActivity("SayHello", task.WithActivityInput(city))
	}
	greetings := make([]string, len(pending))
	for i, activity := range pending {
		if err := activity.Await(&greetings[i]); err != nil {
			return nil, err
		}
	}
	ctx.SetCustomStatus("completed")
	return greetings, nil
}

func SayHello(_ context.Context, city string) (string, error) {
	return fmt.Sprintf("Hello, %s!", city), nil
}

func StartGreetings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var input GreetingRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}
	if err := input.validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	client, ok := durabletask.ClientFromContext(r.Context())
	if !ok {
		http.Error(w, "durable client unavailable", http.StatusInternalServerError)
		return
	}
	id, err := client.ScheduleNewOrchestration(r.Context(), "FanOutGreetings", api.WithInput(input))
	if err != nil {
		http.Error(w, "could not start orchestration", http.StatusInternalServerError)
		return
	}
	_ = client.WriteCheckStatusResponse(w, r, string(id))
}

func GetGreetingStatus(w http.ResponseWriter, r *http.Request) {
	client, ok := durabletask.ClientFromContext(r.Context())
	if !ok {
		http.Error(w, "durable client unavailable", http.StatusInternalServerError)
		return
	}
	id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	meta, err := client.FetchOrchestrationMetadata(r.Context(), api.InstanceID(id))
	if errors.Is(err, api.ErrInstanceNotFound) {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "could not get orchestration status", http.StatusInternalServerError)
		return
	}
	_ = durabletask.WriteStatusResponse(w, meta)
}

func newApp() *sdk.App {
	app := sdk.FunctionApp()
	durable := durabletask.Middleware(
		durabletask.WithOrchestrator("FanOutGreetings", FanOutGreetings),
		durabletask.WithActivity("SayHello", SayHello),
	)
	app.Use(durable)
	app.HTTP("StartGreetings", StartGreetings,
		sdk.WithMethods("POST"), sdk.WithRoute("greetings"), sdk.WithAuth("function"),
		durabletask.ClientInput())
	app.HTTP("GetGreetingStatus", GetGreetingStatus,
		sdk.WithMethods("GET"), sdk.WithRoute("greetings/{instanceId}"), sdk.WithAuth("function"),
		durabletask.ClientInput())
	return app
}

func main() {
	worker.Start(newApp())
}
