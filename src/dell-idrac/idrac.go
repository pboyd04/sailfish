package dell_idrac

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sync"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/actionhandler"
	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"
	"github.com/superchalupa/sailfish/src/dell-resources/attributes"
	"github.com/superchalupa/sailfish/src/dell-resources/registries"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
	"github.com/superchalupa/sailfish/src/ocp/session"
	"github.com/superchalupa/sailfish/src/ocp/stdcollections"
	"github.com/superchalupa/sailfish/src/ocp/telemetryservice"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"

	//"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"github.com/superchalupa/sailfish/src/stdmeta"
	"github.com/superchalupa/sailfish/src/uploadhandler"

	// register all the DM events that are not otherwise pulled in
	_ "github.com/superchalupa/sailfish/src/dell-resources/dm_event"

	// goal is to get rid of the _ in front of each of these....
	dell_ec "github.com/superchalupa/sailfish/src/dell-ec"
	"github.com/superchalupa/sailfish/src/dell-idrac/chassis/system_chassis"
	"github.com/superchalupa/sailfish/src/dell-idrac/managers/idrac_embedded"
	"github.com/superchalupa/sailfish/src/dell-idrac/storage"
	"github.com/superchalupa/sailfish/src/dell-idrac/system"
	/*
		// all these are TODO:
				storage_enclosure "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/enclosure"
				storage_instance "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage"
				storage_controller "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/controller"
				storage_drive "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/drive"
				storage_collection "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/storage_collection"
				storage_volume "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/volume"
				storage_volume_collection "github.com/superchalupa/sailfish/src/dell-resources/systems/system.embedded/storage/volume_collection"
	*/)

type ocp struct {
	configChangeHandler func()
}

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func (o *ocp) ConfigChangeHandler() { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) *ocp {
	logger = logger.New("module", "ec")
	self := &ocp{}

	// These three all set up a waiter for the root service to appear, so init root service after.
	actionhandler.Setup(ctx, ch, eb)
	event.Setup(ch, eb)
	domain.StartInjectService(logger, eb)
	arService, _ := ar_mapper2.StartService(ctx, logger, cfgMgr, cfgMgrMu, eb)
	actionSvc := ah.StartService(ctx, logger, ch, eb)
	uploadSvc := uploadhandler.StartService(ctx, logger, ch, eb)
	am2Svc, _ := awesome_mapper2.StartService(ctx, logger, eb)
	ardumpSvc, _ := attributes.StartService(ctx, logger, eb)
	pumpSvc := dell_ec.NewPumpActionSvc(ctx, logger, eb)

	// the package for this is going to change, but this is what makes the various mappers and view functions available
	instantiateSvc := testaggregate.New(ctx, logger, cfgMgr, cfgMgrMu, ch)
	evtSvc := eventservice.New(ctx, cfgMgr, cfgMgrMu, d, instantiateSvc, actionSvc, uploadSvc)
	testaggregate.RegisterWithURI(instantiateSvc)
	testaggregate.RegisterPublishEvents(instantiateSvc, evtSvc)
	testaggregate.RegisterAM2(instantiateSvc, am2Svc)
	testaggregate.RegisterPumpAction(instantiateSvc, actionSvc, pumpSvc)
	ar_mapper2.RegisterARMapper(instantiateSvc, arService)
	attributes.RegisterController(instantiateSvc, ardumpSvc)
	stdmeta.RegisterFormatters(instantiateSvc, d)
	registries.RegisterAggregate(instantiateSvc)
	stdcollections.RegisterAggregate(instantiateSvc)
	session.RegisterAggregate(instantiateSvc)
	attributes.RegisterAggregate(instantiateSvc)
	eventservice.RegisterAggregate(instantiateSvc)
	idrac_embedded.RegisterAggregate(instantiateSvc)
	system_chassis.RegisterAggregate(instantiateSvc)
	storage.RegisterAggregate(instantiateSvc)
	system.RegisterAggregate(instantiateSvc)
	telemetryservice.RegisterAggregate(instantiateSvc)

	AddChassisInstantiate(logger, instantiateSvc)
	//instantiate power resources
	RegisterAggregate(instantiateSvc)

	// ignore unused for now
	_ = actionSvc

	awesome_mapper2.AddFunction("find_uris_with_basename", func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return nil, errors.New("need to specify uri to match")
		}
		p, ok := args[0].(string)
		if !ok {
			return nil, errors.New("need to specify uri to match")
		}

		return d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == p }), nil
	})

	// add mapper helper to instantiate
	awesome_mapper2.AddFunction("instantiate", func(args ...interface{}) (interface{}, error) {
		if len(args) < 1 {
			return nil, errors.New("need to specify cfg section to instantiate")
		}
		cfgStr, ok := args[0].(string)
		if !ok {
			return nil, errors.New("need to specify cfg section to instantiate")
		}

		params := map[string]interface{}{}
		var key string
		for i, val := range args[1:] {
			if i%2 == 0 {
				key, ok = val.(string)
				if !ok {
					return nil, fmt.Errorf("got a non-string key value: %s", key)
				}
			} else {
				params[key] = val
			}
		}

		instantiateSvc.Instantiate(cfgStr, params)
		return true, nil
	})

	//*********************************************************************
	//  /redfish/v1
	//*********************************************************************
	rooturi := "/redfish/v1"
	_, rootView, _ := instantiateSvc.Instantiate("rootview",
		map[string]interface{}{
			"rooturi":                   rooturi,
			"submit_test_metric_report": view.Action(telemetryservice.MakeSubmitTestMetricReport(eb)),
		})

	//*********************************************************************
	//  Standard redfish roles
	//*********************************************************************
	// TODO: convert to instantiate
	stdcollections.AddStandardRoles(ctx, rootView.GetUUID(), rootView.GetURI(), ch)

	//*********************************************************************
	// /redfish/v1/Sessions
	//*********************************************************************
	_, sessionSvcVw, _ := instantiateSvc.Instantiate("sessionservice", map[string]interface{}{})
	session.SetupSessionService(instantiateSvc, sessionSvcVw, ch, eb)
	instantiateSvc.Instantiate("sessioncollection", map[string]interface{}{})

	//*********************************************************************
	// /redfish/v1/EventService
	//*********************************************************************
	evtSvc.StartEventService(ctx, logger, instantiateSvc, map[string]interface{}{})

	//*********************************************************************
	// /redfish/v1/Registries
	//*********************************************************************
	for regName, location := range map[string]interface{}{
		"idrac_registry":    []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/Messages/EEMIRegistry.v1_5_0.json"}},
		"base_registry":     []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/BaseMessages/BaseRegistry.v1_0_0.json", "PublicationUri": "http://www.dmtf.org/sites/default/files/standards/documents/DSP8011_1.0.0a.json"}},
		"mgr_attr_registry": []map[string]string{{"Language": "En", "Uri": "/redfish/v1/Registries/ManagerAttributeRegistry/ManagerAttributeRegistry.v1_0_0.json"}},
	} {
		instantiateSvc.Instantiate(regName, map[string]interface{}{"location": location})
	}
	return self
}
