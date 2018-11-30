package update_service

// this file should define the BMC Manager object golang data structures where
// we put all the data, plus the aggregate that pulls the data.  actual data
// population should happen in an impl class. ie. no dbus calls in this file

import (
    "context"
    "sync"

    "github.com/superchalupa/sailfish/src/ocp/view"
    "github.com/superchalupa/sailfish/src/log"
    domain "github.com/superchalupa/sailfish/src/redfishresource"
    "github.com/superchalupa/sailfish/src/ocp/testaggregate"

    eh "github.com/looplab/eventhorizon"
    "github.com/spf13/viper"
)

func EnhanceAggregate(ctx context.Context, v *view.View, baseView *view.View, ch eh.CommandHandler) {
    ch.HandleCommand(ctx,
        &domain.UpdateRedfishResourceProperties{
            ID: baseView.GetUUID(),
            Properties: map[string]interface{}{
                "UpdateService": map[string]interface{}{"@odata.id": v.GetURI()},
            },
        })
}

func RegisterAggregate(s *testaggregate.Service) {
    s.RegisterAggregateFunction("update_service",
        func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
            return []eh.Command{
                &domain.CreateRedfishResource{
                    ID:          vw.GetUUID(),
                    ResourceURI: vw.GetURI(),
                    Type:        "#UpdateService.v1_0_0.UpdateService",
                    Context:     params["rooturi"].(string) + "/$metadata#UpdateService.UpdateService",
                    Privileges: map[string]interface{}{
                        "GET":    []string{"Login"},
                        "POST":   []string{}, // cannot create sub objects
                        "PUT":    []string{},
                        "PATCH":  []string{"ConfigureManager"},
                        "DELETE": []string{}, // can't be deleted
                    },
                    Properties: map[string]interface{}{
                        "ServiceEnabled": true, //TODO
                        "Id":             "UpdateService",
                        "Name":           "Update Service",
                        "Description":    "Represents the properties for the Update Service",
                        "Status": map[string]interface{}{
                            "State":  "Enabled", //TODO
                            "Health": "OK",      //TODO
                        },

                        "Attributes@meta": vw.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),

                        "FirmwareInventory": map[string]interface{}{
                            "@odata.id": vw.GetURI() + "/FirmwareInventory",
                        },
                        "Actions": map[string]interface{}{
                            "Oem": map[string]interface{}{
                                "#DellUpdateService.v1_0_0.DellUpdateService.Reset": map[string]interface{}{
                                    "target": "/redfish/v1/UpdateService/Actions/Oem/DellUpdateService.Reset",//vw.GetActionURI("update.reset"), temporarily hardcoded until pumpservice can be parsed in instantiate
                                },
                                "UpdateService.v1_0_0#EID_674_UpdateService.Reset": map[string]interface{}{
                                    "target": "/redfish/v1/UpdateService/Actions/Oem/EID_674_UpdateService.Reset",//vw.GetActionURI("update.eid674.reset"), temporarily hardcoded until pumpservice can be parsed in instantiate
                                },
                                "#DellUpdateService.v1_0_0.DellUpdateService.Syncup": map[string]interface{}{
                                    "target": "/redfish/v1/UpdateService/Actions/Oem/DellUpdateService.Syncup",//vw.GetActionURI("update.syncup"), temporarily hardcoded until pumpservice can be parsed in instantiate
                                },
                                "UpdateService.v1_0_0#EID_674_UpdateService.Syncup": map[string]interface{}{
                                    "target": "/redfish/v1/UpdateService/Actions/Oem/EID_674_UpdateService.Syncup",//vw.GetActionURI("update.eid674.syncup"), temporarily hardcoded until pumpservice can be parsed in instantiate
                                },
                            },
                        },
                    }},
            }, nil
    })

    // Create Firmware Inventory Collection
    s.RegisterAggregateFunction("update_service_firmwareinventory",
        func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
            return []eh.Command{
                &domain.CreateRedfishResource{
                    ID: vw.GetUUID(),
                    ResourceURI: vw.GetURI(),
                    Type:        "#SoftwareInventoryCollection.SoftwareInventoryCollection",
                    Context:     params["rooturi"].(string) + "/$metadata#SoftwareInventoryCollection.SoftwareInventoryCollection",
                    Plugin:      "GenericUploadHandler",
                    Privileges: map[string]interface{}{
                        "GET":    []string{"Login"},
                        "POST":   []string{"ConfigureManager"},
                        "PUT":    []string{}, // Read Only
                        "PATCH":  []string{}, // Read Only
                        "DELETE": []string{}, // can't be deleted
                    },
                    Properties: map[string]interface{}{
                        "Name":        "Firmware Inventory Collection",
                        "Description": "Collection of Firmware Inventory",
                        "Members@meta": vw.Meta(view.GETProperty("firmware_inventory_members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
                        "Members@odata.count@meta": vw.Meta(view.GETProperty("firmware_inventory_members"), view.GETFormatter("count"), view.GETModel("default")),
                    }},
            }, nil
    })

    return
}
