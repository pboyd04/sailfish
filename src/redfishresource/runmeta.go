package domain

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
)

func (rrp *RedfishResourceProperty) MarshalJSON() ([]byte, error) {
	rrp.RLock()
	defer rrp.RUnlock()
	return json.Marshal(rrp.Value)
}

func NewGet(ctx context.Context, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty) error {
	opts := nuEncOpts{
		request: nil,
		process: nuGETfn,
		root:    true,
	}

	return rrp.RunMetaFunctions(ctx, agg, auth, opts)
}

func NewPatch(ctx context.Context, agg *RedfishResourceAggregate, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, body interface{}) error {
	// Paste in redfish spec stuff here
	// 200 if anything succeeds, 400 if everything fails
	opts := nuEncOpts{
		request: body,
		process: nuPATCHfn,
		root:    true,
	}

	return rrp.RunMetaFunctions(ctx, agg, auth, opts)
}

type nuProcessFn func(context.Context, *RedfishResourceProperty, *RedfishAuthorizationProperty, nuEncOpts) error

type nuEncOpts struct {
	root    bool
	request interface{}
	present bool
	process nuProcessFn
}

func nuGETfn(ctx context.Context, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, opts nuEncOpts) error {
	meta_t, ok := rrp.Meta["GET"].(map[string]interface{})
	if !ok {
		return nil // it's not really an "error" we need upper layers to care about
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		return errors.New("No plugin in GET")
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		return errors.New("No plugin named(" + pluginName + ") for GET")
	}

	if plugin, ok := plugin.(PropGetter); ok {
		rrp.Ephemeral = true
		err = plugin.PropertyGet(ctx, auth, rrp, meta_t)
	}
	return err
}

type PropGetter interface {
	PropertyGet(context.Context, *RedfishAuthorizationProperty, *RedfishResourceProperty, map[string]interface{}) error
}

type PropPatcher interface {
	PropertyPatch(context.Context, *RedfishAuthorizationProperty, *RedfishResourceProperty, interface{}, map[string]interface{}) error
}

func nuPATCHfn(ctx context.Context, rrp *RedfishResourceProperty, auth *RedfishAuthorizationProperty, opts nuEncOpts) error {
	if !opts.present {
		return nuGETfn(ctx, rrp, auth, opts)
	}

	meta_t, ok := rrp.Meta["PATCH"].(map[string]interface{})
	if !ok {
		ContextLogger(ctx, "property_process").Debug("No PATCH meta", "meta", meta_t)
		return nuGETfn(ctx, rrp, auth, opts)
	}

	pluginName, ok := meta_t["plugin"].(string)
	if !ok {
		ContextLogger(ctx, "property_process").Debug("No pluginname in patch meta", "meta", meta_t)
		return nuGETfn(ctx, rrp, auth, opts)
	}

	plugin, err := InstantiatePlugin(PluginType(pluginName))
	if err != nil {
		ContextLogger(ctx, "property_process").Debug("No such pluginname", "pluginName", pluginName)
		return nuGETfn(ctx, rrp, auth, opts)
	}

	//ContextLogger(ctx, "property_process").Debug("getting property: PATCH", "value", fmt.Sprintf("%v", rrp.Value), "plugin", plugin)
	if plugin, ok := plugin.(PropPatcher); ok {
		//defer ContextLogger(ctx, "property_process").Debug("AFTER getting property: PATCH - type assert success", "value", fmt.Sprintf("%v", rrp.Value))
		return plugin.PropertyPatch(ctx, auth, rrp, opts.request, meta_t)
	} else {
		panic("coding error: the plugin " + pluginName + " does not implement the Property Patching API")
	}
}

type stopProcessing interface {
	ShouldStop() bool
}

// this should always be string/int/float, or map/slice. There should never be pointers or other odd data structures in a rrp.
func Flatten(thing interface{}) interface{} {
	// if it's an rrp, return the value
	if vp, ok := thing.(*RedfishResourceProperty); ok {
		v := vp.Value
		if vp.Ephemeral {
			vp.Value = nil
		}
		return Flatten(v)
	}

	// recurse through maps or slices and recursively call helper on them
	val := reflect.ValueOf(thing)
	switch k := val.Kind(); k {
	//	case reflect.Ptr:
	//		fmt.Printf("PTR!\n")

	case reflect.Map:
		// everything inside of a redfishresourceproperty should fit into a map[string]interface{}
		ret := map[string]interface{}{}
		for _, k := range val.MapKeys() {
			s, ok := k.Interface().(string)
			v := val.MapIndex(k)
			if ok && v.IsValid() {
				ret[s] = Flatten(v.Interface())
			}
		}
		return ret

	case reflect.Slice:
		// squash every type of array into an []interface{}
		ret := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			sliceVal := val.Index(i)
			if sliceVal.IsValid() {
				ret[i] = Flatten(sliceVal.Interface())
			}
		}
		return ret

	default:
		return thing
	}

	return nil
}

func (rrp *RedfishResourceProperty) RunMetaFunctions(ctx context.Context, agg *RedfishResourceAggregate, auth *RedfishAuthorizationProperty, e nuEncOpts) (err error) {
	rrp.Lock()
	defer rrp.Unlock()

	err = e.process(ctx, rrp, auth, e)
	if a, ok := err.(stopProcessing); ok && a.ShouldStop() {
		return
	}

	helperError := helper(ctx, agg, auth, e, rrp.Value)
	// TODO: need to collect messages here

	if err != nil {
		return err
	} else {
		return helperError
	}
}

type propertyExtMessages interface {
	GetPropertyExtendedMessages() []interface{}
}

type objectExtMessages interface {
	GetObjectExtendedMessages() []interface{}
}

type objectErrMessages interface {
	GetObjectErrorMessages() []interface{}
}

func helper(ctx context.Context, agg *RedfishResourceAggregate, auth *RedfishAuthorizationProperty, e nuEncOpts, v interface{}) error {
	// handle special case of RRP inside RRP.Value of parent
	if vp, ok := v.(*RedfishResourceProperty); ok {
		return vp.RunMetaFunctions(ctx, agg, auth, e)
	}

	objectErrorMessages := []interface{}{}
	objectExtendedMessages := []interface{}{}

	// recurse through maps or slices and recursively call helper on them
	val := reflect.ValueOf(v)
	switch k := val.Kind(); k {
	case reflect.Map:

		elemType := val.Type().Elem()
		if e.root {
			annotatedKey := "@Message.ExtendedInfo"
			val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.Value{})
			val.SetMapIndex(reflect.ValueOf("error"), reflect.Value{})
		}

		for _, k := range val.MapKeys() {
			// first scrub any old extended messages
			if strK, ok := k.Interface().(string); ok {
				annotatedKey := strK + "@Message.ExtendedInfo"
				val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.Value{})
			}

			newEncOpts := nuEncOpts{
				request: e.request,
				present: e.present,
				process: e.process,
				root:    false,
			}

			requestBody, ok := newEncOpts.request.(map[string]interface{})
			newEncOpts.present = ok
			if newEncOpts.present {
				newEncOpts.request, newEncOpts.present = requestBody[k.Interface().(string)]
			}

			mapVal := val.MapIndex(k)
			if mapVal.IsValid() {
				err := helper(ctx, agg, auth, newEncOpts, mapVal.Interface())
				if err == nil {
					continue
				}
				// annotate at this level
				propertyExtendedMessages := []interface{}{}
				if e, ok := err.(propertyExtMessages); ok {
					propertyExtendedMessages = append(propertyExtendedMessages, e.GetPropertyExtendedMessages()...)
				}
				// things to kick up a level
				if e, ok := err.(objectExtMessages); ok {
					objectExtendedMessages = append(objectExtendedMessages, e.GetObjectExtendedMessages()...)
				}
				if e, ok := err.(objectErrMessages); ok {
					objectErrorMessages = append(objectErrorMessages, e.GetObjectErrorMessages()...)
				}

				// TODO: add generic annotation support

				if len(propertyExtendedMessages) > 0 {
					if strK, ok := k.Interface().(string); ok {
						annotatedKey := strK + "@Message.ExtendedInfo"
						if compatible(reflect.TypeOf(propertyExtendedMessages), elemType) {
							val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.ValueOf(propertyExtendedMessages))
						}
					}
				}
			}
		}

		if e.root && len(objectExtendedMessages) > 0 {
			annotatedKey := "@Message.ExtendedInfo"
			if compatible(reflect.TypeOf(objectExtendedMessages), val.Type().Elem()) {
				val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.ValueOf(objectExtendedMessages))
			}
		}

		if e.root && len(objectErrorMessages) > 0 {
			if agg != nil {
				agg.StatusCode = 400
			}
			annotatedKey := "error"
			value := map[string]interface{}{
				"code":                  "Base.1.0.GeneralError",
				"message":               "A general error has occurred. See ExtendedInfo for more information.",
				"@Message.ExtendedInfo": objectErrorMessages,
			}
			if compatible(reflect.TypeOf(value), val.Type().Elem()) {
				val.SetMapIndex(reflect.ValueOf(annotatedKey), reflect.ValueOf(value))
			}
		} else {
			if agg != nil {
				agg.StatusCode = 200
			}
		}

	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			sliceVal := val.Index(i)
			if sliceVal.IsValid() {
				err := helper(ctx, agg, auth, e, sliceVal.Interface())
				if err == nil {
					continue
				}

				// things to kick up a level
				if e, ok := err.(objectExtMessages); ok {
					objectExtendedMessages = append(objectExtendedMessages, e.GetObjectExtendedMessages()...)
				}
				if e, ok := err.(objectErrMessages); ok {
					objectErrorMessages = append(objectErrorMessages, e.GetObjectErrorMessages()...)
				}

				// TODO: do annotations make sense here?
			}
		}
	}

	return &CombinedPropObjInfoError{
		ObjectExtendedInfoMessages:  *NewObjectExtendedInfoMessages(objectExtendedMessages),
		ObjectExtendedErrorMessages: *NewObjectExtendedErrorMessages(objectErrorMessages),
	}

}

func compatible(actual, expected reflect.Type) bool {
	if actual == nil {
		k := expected.Kind()
		return k == reflect.Chan ||
			k == reflect.Func ||
			k == reflect.Interface ||
			k == reflect.Map ||
			k == reflect.Ptr ||
			k == reflect.Slice
	}
	return actual.AssignableTo(expected)
}
