package powercontrol

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#Power.v1_0_2.PowerControl",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":                     "System Power Control",
				"MemberId":                 "PowerControl",
				"PowerAvailableWatts@meta": v.Meta(view.PropGET("available_watts")), // TODO
				"PowerCapacityWatts@meta":  v.Meta(view.PropGET("capacity_watts")),  //System.Chassis.1#ChassisPower.1#SystemInputMaxPowerCapacity
				"PowerConsumedWatts@meta":  v.Meta(view.PropGET("consumed_watts")),  // TODO

				"Oem": map[string]interface{}{
					"EnergyConsumptionStartTime@meta": v.Meta(view.PropGET("energy_consumption_start_time")),
					"EnergyConsumptionkWh@meta":       v.Meta(view.PropGET("energy_consumption_kwh")),
					"HeadroomWatts@meta":              v.Meta(view.PropGET("headroom_watts")), // TODO
					"MaxPeakWatts@meta":               v.Meta(view.PropGET("max_peak_watts")),
					"MaxPeakWattsTime@meta":           v.Meta(view.PropGET("max_peak_watts_time")),
					"MinPeakWatts@meta":               v.Meta(view.PropGET("min_peak_watts")),
					"MinPeakWattsTime@meta":           v.Meta(view.PropGET("min_peak_watts_time")),
					"PeakHeadroomWatts@meta":          v.Meta(view.PropGET("peak_headroom_watts")), // TODO
				},
				"PowerLimit": map[string]interface{}{
					"LimitInWatts@meta": v.Meta(view.PropGET("limit_in_watts")),
				},
				"PowerMetrics": map[string]interface{}{
					"AverageConsumedWatts": 0,
					"IntervalInMin":        0,
					"MaxConsumedWatts":     0,
					"MinConsumedWatts":     0,
				},
				"RelatedItem@meta":             v.Meta(view.GETProperty("power_related_items"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
				"RelatedItem@odata.count@meta": v.Meta(view.GETProperty("power_related_items"), view.GETFormatter("count"), view.GETModel("default")),
			},
		})
}
