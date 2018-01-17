package session

import (
	"context"
	"fmt"
	"net/http"

	domain "github.com/superchalupa/go-redfish/internal/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

type AddUserDetails struct {
	OnUserDetails func(userName string, privileges []string) http.Handler
}

func (a *AddUserDetails) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// pretend to parse token! for now...
	// TODO: actually parse the token
	a.OnUserDetails("root", []string{"Admin", "Unauthenticated"}).ServeHTTP(rw, req)
}

func SetupSessionService(ctx context.Context, rootID eh.UUID, ew *utils.EventWaiter, ch eh.CommandHandler, eb eh.EventBus) (aud *AddUserDetails) {
	fmt.Printf("SetupSessionService\n")

	// register our command
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, commandHandler: ch, eventWaiter: ew} })

	// set up the return value since we already know it
	aud = &AddUserDetails{}

	l, err := ew.Listen(ctx, func(event eh.Event) bool {
		if event.EventType() != domain.RedfishResourceCreated {
			return false
		}
		if data, ok := event.Data().(*domain.RedfishResourceCreatedData); ok {
			if data.ResourceURI == "/redfish/v1/" {
				return true
			}
		}
		return false
	})
	if err != nil {
		return
	}

	// wait for the root object to be created, then enhance it. Oneshot for now.
	go func() {
		defer l.Close()

		_, err := l.Wait(ctx)
		if err != nil {
			fmt.Printf("Error waiting for event: %s\n", err.Error())
			return
		}

		// Create SessionService aggregate
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				ResourceURI: "/redfish/v1/SessionService",
				Type:        "#SessionService.v1_0_2.SessionService",
				Context:     "/redfish/v1/$metadata#SessionService",
				Privileges: map[string]interface{}{
					"GET":    []string{"ConfigureManager"},
					"POST":   []string{"ConfigureManager"},
					"PUT":    []string{"ConfigureManager"},
					"PATCH":  []string{"ConfigureManager"},
					"DELETE": []string{},
				},
				Properties: map[string]interface{}{
					"Id":          "SessionService",
					"Name":        "Session Service",
					"Description": "Session Service",
					"Status": map[string]interface{}{
						"State":  "Enabled",
						"Health": "OK",
					},
					"ServiceEnabled": true,
					"SessionTimeout": 30,
					"Sessions": map[string]interface{}{
						"@odata.id": "/redfish/v1/SessionService/Sessions",
					},
					"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
				}})

		// Create Sessions Collection
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				Plugin:      "SessionService",
				ResourceURI: "/redfish/v1/SessionService/Sessions",
				Type:        "#SessionCollection.SessionCollection",
				Context:     "/redfish/v1/$metadata#SessionService/Sessions/$entity",
				Privileges: map[string]interface{}{
					"GET":    []string{"ConfigureManager"},
					"POST":   []string{"Unauthenticated"},
					"PUT":    []string{"ConfigureManager"},
					"PATCH":  []string{"ConfigureManager"},
					"DELETE": []string{"ConfigureSelf"},
				},
				Collection: true,
				Properties: map[string]interface{}{
					"Name":                "Session Collection",
					"Members@odata.count": 0,
					"Members":             []map[string]interface{}{},
					"@Redfish.Copyright":  "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
				}})

		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID: rootID,
				Properties: map[string]interface{}{
					"SessionService": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService"},
					"Links":          map[string]interface{}{"Sessions": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService/Sessions"}},
				},
			})
	}()

	return
}