package domain

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"
)

func init() {
	// implemented
	eh.RegisterCommand(func() eh.Command { return &DELETE{} })

	// GET/PATCH registered in the handler as they pub on http event bus

	// TODO: not yet implemented
	eh.RegisterCommand(func() eh.Command { return &PUT{} })
	eh.RegisterCommand(func() eh.Command { return &POST{} })
	eh.RegisterCommand(func() eh.Command { return &HEAD{} })
	eh.RegisterCommand(func() eh.Command { return &OPTIONS{} })
}

const (
	DELETECommand = eh.CommandType("http:RedfishResource:DELETE")

	PUTCommand     = eh.CommandType("http:RedfishResource:PUT")
	PATCHCommand   = eh.CommandType("http:RedfishResource:PATCH")
	POSTCommand    = eh.CommandType("http:RedfishResource:POST")
	HEADCommand    = eh.CommandType("http:RedfishResource:HEAD")
	OPTIONSCommand = eh.CommandType("http:RedfishResource:OPTIONS")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&DELETE{})

var _ = eh.Command(&PUT{})
var _ = eh.Command(&PATCH{})
var _ = eh.Command(&POST{})
var _ = eh.Command(&HEAD{})
var _ = eh.Command(&OPTIONS{})

// HTTP DELETE Command
type DELETE struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

func (c *DELETE) AggregateType() eh.AggregateType { return AggregateType }
func (c *DELETE) AggregateID() eh.UUID            { return c.ID }
func (c *DELETE) CommandType() eh.CommandType     { return DELETECommand }
func (c *DELETE) SetAggID(id eh.UUID)             { c.ID = id }
func (c *DELETE) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *DELETE) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// TODO: "Services may return a representation of the just deleted resource in the response body."
	// - can create a new CMD for GET with an identical CMD ID. Is that cheating?
	// TODO: return http 405 status for undeletable objects. right now we use privileges

	//data.Results, _ = ProcessDELETE(ctx, a.Properties, c.Body)

	// send event to trigger delete
	a.PublishEvent(eh.NewEvent(RedfishResourceRemoved, &RedfishResourceRemovedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
	}, time.Now()))

	// send http response
	a.PublishEvent(eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{},
		StatusCode: 200,
		Headers:    map[string]string{},
	}, time.Now()))
	return nil
}

// HTTP PATCH Command
type PATCH struct {
	ID           eh.UUID `json:"id"`
	CmdID        eh.UUID `json:"cmdid"`
	HTTPEventBus eh.EventBus

	Body map[string]interface{} `eh:"optional"`
	auth *RedfishAuthorizationProperty
}

func (c *PATCH) AggregateType() eh.AggregateType { return AggregateType }
func (c *PATCH) AggregateID() eh.UUID            { return c.ID }
func (c *PATCH) CommandType() eh.CommandType     { return PATCHCommand }
func (c *PATCH) SetAggID(id eh.UUID)             { c.ID = id }
func (c *PATCH) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *PATCH) SetUserDetails(a *RedfishAuthorizationProperty) string {
	c.auth = a
	return "checkMaster"
}
func (c *PATCH) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.Body)
	return nil
}
func (c *PATCH) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// set up the base response data
	data := &HTTPCmdProcessedData{
		CommandID: c.CmdID,
		Headers:   map[string]string{},
	}
	for k, v := range a.Headers {
		data.Headers[k] = v
	}

	a.ResultsCacheMu.Lock()
	defer a.ResultsCacheMu.Unlock()

	NewPatch(ctx, a, &a.Properties, c.auth, c.Body)
	data.StatusCode = a.StatusCode
	data.Results = Flatten(a.Properties.Value)

	c.HTTPEventBus.PublishEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))

	return nil
}

// HTTP POST Command
type POST struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
	Body  map[string]interface{}
}

func (c *POST) AggregateType() eh.AggregateType { return AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.Body)
	return nil
}
func (c *POST) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.PublishEvent(eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"FOO": "BAR"},
		StatusCode: 200,
		Headers:    map[string]string{},
	}, time.Now()))
	return nil
}

// HTTP PUT Command
type PUT struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
	Body  map[string]interface{}
}

func (c *PUT) AggregateType() eh.AggregateType { return AggregateType }
func (c *PUT) AggregateID() eh.UUID            { return c.ID }
func (c *PUT) CommandType() eh.CommandType     { return PUTCommand }
func (c *PUT) SetAggID(id eh.UUID)             { c.ID = id }
func (c *PUT) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *PUT) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.Body)
	return nil
}
func (c *PUT) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.PublishEvent(eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"ERROR": "This method not yet implemented"},
		StatusCode: 501, // Not implemented
		Headers:    map[string]string{},
	}, time.Now()))
	return nil
}

// HTTP HEAD Command
type HEAD struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

func (c *HEAD) AggregateType() eh.AggregateType { return AggregateType }
func (c *HEAD) AggregateID() eh.UUID            { return c.ID }
func (c *HEAD) CommandType() eh.CommandType     { return HEADCommand }
func (c *HEAD) SetAggID(id eh.UUID)             { c.ID = id }
func (c *HEAD) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *HEAD) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.PublishEvent(eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"ERROR": "This method not yet implemented"},
		StatusCode: 501, // Not implemented
		Headers:    map[string]string{},
	}, time.Now()))
	return nil
}

// HTTP OPTIONS Command
type OPTIONS struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

func (c *OPTIONS) AggregateType() eh.AggregateType { return AggregateType }
func (c *OPTIONS) AggregateID() eh.UUID            { return c.ID }
func (c *OPTIONS) CommandType() eh.CommandType     { return OPTIONSCommand }
func (c *OPTIONS) SetAggID(id eh.UUID)             { c.ID = id }
func (c *OPTIONS) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *OPTIONS) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.PublishEvent(eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"ERROR": "This method not yet implemented"},
		StatusCode: 501, // Not implemented
		Headers:    map[string]string{},
	}, time.Now()))
	return nil
}
