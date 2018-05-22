package iom_chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func AddView(ctx context.Context, s *model.Service, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          model.GetUUID(s),
			Collection:  false,
			ResourceURI: model.GetOdataID(s),
			Type:        "#Chassis.v1_0_2.Chassis",
			Context:     "/redfish/v1/$metadata#ChassisCollection.ChassisCollection/Members/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":             s.GetProperty("unique_name").(string),
				"ManagedBy@meta": s.Meta(model.PropGET("managed_by")),
				// TODO: "ManagedBy@odata.count": 1  (need some infrastructure for this)

				"AssetTag@meta":     s.Meta(model.PropGET("asset_tag")),
				"Description@meta":  s.Meta(model.PropGET("description")),
				"PowerState@meta":   s.Meta(model.PropGET("power_state")),
				"SerialNumber@meta": s.Meta(model.PropGET("serial")),
				"PartNumber@meta":   s.Meta(model.PropGET("part_number")),
				"ChassisType@meta":  s.Meta(model.PropGET("chassis_type")),
				"Model@meta":        s.Meta(model.PropGET("model")),
				"Name@meta":         s.Meta(model.PropGET("name")),
				"Manufacturer@meta": s.Meta(model.PropGET("manufacturer")),

				"SKU":          "",
				"IndicatorLED": "Lit",
				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "Enabled",
					"Health":       "OK",
				},
				"Oem": map[string]interface{}{
					"Dell": map[string]interface{}{
						"ServiceTag@meta":      s.Meta(model.PropGET("service_tag")),
						"InstPowerConsumption": 24,
						"OemChassis": map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/" + s.GetProperty("unique_name").(string) + "/Attributes",
						},
						"OemIOMConfiguration": map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/" + s.GetProperty("unique_name").(string) + "/IOMConfiguration",
						},
					},
				},

				"Actions": map[string]interface{}{
					"#Chassis.Reset": map[string]interface{}{
						"ResetType@Redfish.AllowableValues": []string{
							"On",
							"GracefulShutdown",
							"GracefulRestart",
						},
						"target": "/redfish/v1/Chassis/" + s.GetProperty("unique_name").(string) + "/Actions/Chassis.Reset",
					},
					"Oem": map[string]interface{}{
						"DellChassis.v1_0_0#DellChassis.ResetPeakPowerConsumption": map[string]interface{}{
							"target": "/redfish/v1/Chassis/" + s.GetProperty("unique_name").(string) + "/Actions/Oem/DellChassis.ResetPeakPowerConsumption",
						},
						"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
							"target": "/redfish/v1/Chassis/" + s.GetProperty("unique_name").(string) + "/Actions/Oem/DellChassis.VirtualReseat",
						},
					},
				},
			}})
}
