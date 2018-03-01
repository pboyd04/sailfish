// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build openbmc

package obmc

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	"github.com/superchalupa/go-redfish/plugins"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
	mydbus "github.com/superchalupa/go-redfish/plugins/dbus"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	"github.com/superchalupa/go-redfish/plugins/ocp/bmc"
	"github.com/superchalupa/go-redfish/plugins/ocp/chassis"
	"github.com/superchalupa/go-redfish/plugins/ocp/protocol"
)

func init() {
	domain.RegisterInitFN(InitService)
}

const (
	DbusTimeout time.Duration = 1
)

func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// initial implementation is one BMC, one Chassis, and one System. If we
	// expand beyond that, we need to adjust stuff here.

	bmcSvc, _ := bmc.NewBMCService(
		bmc.WithURIName("OBMC"),
	)

	bmcSvc.Service.ApplyOption(
		plugins.UpdateProperty("name", "OBMC Simulation"),
		plugins.UpdateProperty("description", "The most open source BMC ever."),
		plugins.UpdateProperty("model", "Michaels RAD BMC"),
		plugins.UpdateProperty("timezone", "-05:00"),
		plugins.UpdateProperty("version", "1.0.0"),
	)
	plugins.OnURICreated(ctx, ew, "/redfish/v1/Managers", func() { bmcSvc.AddResource(ctx, ch) })

	time.Sleep(300 * time.Millisecond)
	prot, _ := protocol.NewNetProtocols(
		protocol.WithBMC(bmcSvc),
		protocol.WithProtocol("HTTPS", true, 443, nil),
		protocol.WithProtocol("HTTP", false, 80, nil),
		protocol.WithProtocol("IPMI", false, 623, nil),
		protocol.WithProtocol("SSH", false, 22, nil),
		protocol.WithProtocol("SNMP", false, 161, nil),
		protocol.WithProtocol("TELNET", false, 23, nil),
		protocol.WithProtocol("SSDP", false, 1900,
			map[string]interface{}{"NotifyMulticastIntervalSeconds": 600, "NotifyTTL": 5, "NotifyIPv6Scope": "Site"}),
	)
	prot.AddResource(ctx, ch)

	chas, _ := chassis.NewChassisService(
		ctx,
		chassis.ManagedBy(bmcSvc),
		chassis.WithURIName("1"),
	)
	chas.Service.ApplyOption(
		plugins.UpdateProperty("name", "Catfish System Chassis"),
		plugins.UpdateProperty("chassis_type", "RackMount"),
		plugins.UpdateProperty("model", "YellowCat1000"),
		plugins.UpdateProperty("serial_number", "2M220100SL"),
		plugins.UpdateProperty("sku", "The SKU"),
		plugins.UpdateProperty("part_number", "Part2468"),
		plugins.UpdateProperty("asset_tag", "CATFISHASSETTAG"),
		plugins.UpdateProperty("chassis_type", "RackMount"),
	)

	chas.AddResource(ctx, ch)

	system, _ := NewSystemService(ctx)
	system.AddOBMCSystemResource(ctx, ch)

	domain.RegisterPlugin(func() domain.Plugin { return bmcSvc })
	domain.RegisterPlugin(func() domain.Plugin { return prot })
	domain.RegisterPlugin(func() domain.Plugin { return chas })
	//domain.RegisterPlugin(func() domain.Plugin { return chas.thermalSensors })
	domain.RegisterPlugin(func() domain.Plugin { return system })

	// go ret.runbackgroundstuff(ctx)

	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction(bmcSvc.GetOdataID()+"/Actions/Manager.Reset")))
	if err != nil {
		fmt.Printf("Failed to create event stream processor: %s\n", err.Error())
		return // todo: tear down all the prior event stream processors, too
	}
	sp.RunForever(func(event eh.Event) {
		bus := "org.openbmc.control.Bmc"
		path := "/org/openbmc/control/bmc0"
		intfc := "org.openbmc.control.Bmc"

		fmt.Printf("parse resetType: %s\n", event.Data())
		ad := event.Data().(ah.GenericActionEventData)
		resetType, _ := ad.ActionData.(map[string]interface{})["ResetType"]
		call := "undefined"
		if resetType == "ForceRestart" {
			call = "coldReset"
		}
		if resetType == "GracefulRestart" {
			call = "warmReset"
		}
		fmt.Printf("\tgot: %s\n", resetType)
		if call == "undefined" {
			return
		}

		fmt.Printf("connect to system bus\n")
		statusCode := 200
		statusMessage := "OK"

		conn, err := dbus.SystemBus()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot connect to System Bus: %s\n", err.Error())
			statusCode = 501
			statusMessage = "ERROR: Cannot attach to dbus system bus"
		}

		dh := mydbus.NewDbusHelper(conn, bus, path)
		timedctx, cancel := context.WithTimeout(ctx, time.Duration(DbusTimeout))
		defer cancel()
		_, err = dh.DbusCall(timedctx, 0, intfc+"."+call)
		if err != nil {
			statusCode = 501
			statusMessage = "Internal call failed"
		}

		eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"RESET": statusMessage},
			StatusCode: statusCode,
			Headers:    map[string]string{},
		}, time.Now()))
	})
}