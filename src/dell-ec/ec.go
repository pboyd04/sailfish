// Build tags: only build this for the simulation build. Be sure to note the required blank line after.
// +build ec

package obmc

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/viper"
	"io/ioutil"
	// "github.com/go-yaml/yaml"
	yaml "gopkg.in/yaml.v2"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	"github.com/superchalupa/go-redfish/src/log"
	plugins "github.com/superchalupa/go-redfish/src/ocp"
	"github.com/superchalupa/go-redfish/src/ocp/basicauth"
	"github.com/superchalupa/go-redfish/src/ocp/root"
	"github.com/superchalupa/go-redfish/src/ocp/session"

	attr_prop "github.com/superchalupa/go-redfish/src/dell-resources/attribute-property"
	attr_res "github.com/superchalupa/go-redfish/src/dell-resources/attribute-resource"

	"github.com/superchalupa/go-redfish/src/dell-resources"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis"
	"github.com/superchalupa/go-redfish/src/dell-resources/chassis/iom.slot"
	"github.com/superchalupa/go-redfish/src/dell-resources/managers/cmc.integrated"
)

type ocp struct {
	rootSvc             *root.Service
	sessionSvc          *session.Service
	basicAuthSvc        *basicauth.Service
	configChangeHandler func()
	logger              log.Logger
}

func (o *ocp) GetSessionSvc() *session.Service     { return o.sessionSvc }
func (o *ocp) GetBasicAuthSvc() *basicauth.Service { return o.basicAuthSvc }
func (o *ocp) ConfigChangeHandler()                { o.configChangeHandler() }

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *ocp {
	// initial implementation is one BMC, one Chassis, and one System.
	// Yes, this function is somewhat long, however there really isn't any logic here. If we start getting logic, this needs to be split.

	logger = logger.New("module", "ec")
	self := &ocp{
		logger: logger,
	}

	self.rootSvc, _ = root.New()
	domain.RegisterPlugin(func() domain.Plugin { return self.rootSvc })
	self.rootSvc.AddResource(ctx, ch, eb, ew)
	time.Sleep(1)

	self.sessionSvc, _ = session.New(
		session.Root(self.rootSvc),
	)
	domain.RegisterPlugin(func() domain.Plugin { return self.sessionSvc })
	self.sessionSvc.AddResource(ctx, ch, eb, ew)

	self.basicAuthSvc, _ = basicauth.New()
	domain.RegisterPlugin(func() domain.Plugin { return self.basicAuthSvc })
	self.basicAuthSvc.AddResource(ctx, ch, eb, ew)

	cmc_integrated_1_svc, _ := ec_manager.New(
		ec_manager.WithUniqueName("CMC.Integrated.1"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return cmc_integrated_1_svc })
	cmc_integrated_1_svc.AddView(ctx, ch, eb, ew)
	update1Fn, _ := generic_dell_resource.AddController(ctx, logger.New("module", "Managers/CMC.Integrated.1"), cmc_integrated_1_svc.Service, "Managers/CMC.Integrated.1", ch, eb, ew)

	bmcAttrSvc, _ := attr_res.New(
		attr_res.BaseResource(cmc_integrated_1_svc),
		attr_res.WithURI("/redfish/v1/Managers/CMC.Integrated.1/Attributes"),
		attr_res.WithUniqueName("CMC.Integrated.1.Attributes"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttrSvc })
	bmcAttrSvc.AddView(ctx, ch, eb, ew)

	bmcAttrProp, _ := attr_prop.New(
		attr_prop.BaseResource(bmcAttrSvc),
		attr_prop.WithFQDD("CMC.Integrated.1"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttrProp })
	bmcAttrProp.AddView(ctx, ch, eb, ew)
	bmcAttrProp.AddController(ctx, ch, eb, ew)

	cmc_integrated_2_svc, _ := ec_manager.New(
		ec_manager.WithUniqueName("CMC.Integrated.2"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return cmc_integrated_2_svc })
	cmc_integrated_2_svc.AddView(ctx, ch, eb, ew)
	update2Fn, _ := generic_dell_resource.AddController(ctx, logger.New("module", "Managers/CMC.Integrated.2"), cmc_integrated_2_svc.Service, "Managers/CMC.Integrated.2", ch, eb, ew)

	bmcAttr2Svc, _ := attr_res.New(
		attr_res.BaseResource(cmc_integrated_2_svc),
		attr_res.WithURI("/redfish/v1/Managers/CMC.Integrated.2/Attributes"),
		attr_res.WithUniqueName("CMC.Integrated.2.Attributes"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttr2Svc })
	bmcAttr2Svc.AddView(ctx, ch, eb, ew)

	bmcAttr2Prop, _ := attr_prop.New(
		attr_prop.BaseResource(bmcAttr2Svc),
		attr_prop.WithFQDD("CMC.Integrated.2"),
	)
	domain.RegisterPlugin(func() domain.Plugin { return bmcAttr2Prop })
	bmcAttr2Prop.AddView(ctx, ch, eb, ew)
	bmcAttr2Prop.AddController(ctx, ch, eb, ew)

	for _, iomName := range []string{
		"IOM.Slot.A1",
		"IOM.Slot.A1a",
		"IOM.Slot.A1b",
		"IOM.Slot.A2",
		"IOM.Slot.A2a",
		"IOM.Slot.A2b",
		"IOM.Slot.B1",
		"IOM.Slot.B1a",
		"IOM.Slot.B1b",
		"IOM.Slot.B2",
		"IOM.Slot.B2a",
		"IOM.Slot.B2b",
		"IOM.Slot.C1",
		"IOM.Slot.C2",
	} {
		iom, _ := generic_chassis.New(
			generic_chassis.WithUniqueName(iomName),
			generic_chassis.AddManagedBy(cmc_integrated_1_svc),
		)
		domain.RegisterPlugin(func() domain.Plugin { return iom })
		iom_chassis.AddView(iom, ctx, ch, eb, ew)

		iomAttrSvc, _ := attr_res.New(
			attr_res.BaseResource(iom),
			attr_res.WithURI("/redfish/v1/Chassis/"+iomName+"/Attributes"),
			attr_res.WithUniqueName(iomName+".Attributes"),
		)
		domain.RegisterPlugin(func() domain.Plugin { return iomAttrSvc })
		iomAttrSvc.AddView(ctx, ch, eb, ew)
		iomAttrSvc.AddController(ctx, ch, eb, ew)

		iomAttrProp, _ := attr_prop.New(
			attr_prop.BaseResource(iomAttrSvc),
			attr_prop.WithFQDD(iomName),
		)
		domain.RegisterPlugin(func() domain.Plugin { return iomAttrProp })
		iomAttrProp.AddView(ctx, ch, eb, ew)
		iomAttrProp.AddController(ctx, ch, eb, ew)
	}

	// VIPER Config:
	// pull the config from the YAML file to populate some static config options
	self.configChangeHandler = func() {
		logger.Info("Re-applying configuration from config file.")
		self.sessionSvc.ApplyOption(plugins.UpdateProperty("session_timeout", cfgMgr.GetInt("session.timeout")))

		update1Fn(cfgMgr)
		update2Fn(cfgMgr)
	}
	self.ConfigChangeHandler()

	cfgMgr.SetDefault("main.dumpConfigChanges.filename", "redfish-changed.yaml")
	cfgMgr.SetDefault("main.dumpConfigChanges.enabled", "true")
	dumpViperConfig := func() {
		viperMu.Lock()
		defer viperMu.Unlock()

		dumpFileName := cfgMgr.GetString("main.dumpConfigChanges.filename")
		enabled := cfgMgr.GetBool("main.dumpConfigChanges.enabled")
		if !enabled {
			return
		}

		// TODO: change this to a streaming write (reduce mem usage)
		var config map[string]interface{}
		cfgMgr.Unmarshal(&config)
		output, _ := yaml.Marshal(config)
		_ = ioutil.WriteFile(dumpFileName, output, 0644)
	}

	self.sessionSvc.AddPropertyObserver("session_timeout", func(newval interface{}) {
		viperMu.Lock()
		cfgMgr.Set("session.timeout", newval.(int))
		viperMu.Unlock()
		dumpViperConfig()
	})

	return self
}