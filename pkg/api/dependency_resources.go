package api

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"

	"github.com/Aptomi/aptomi/pkg/engine/resolve"
	"github.com/Aptomi/aptomi/pkg/event"
	"github.com/Aptomi/aptomi/pkg/lang"
	"github.com/Aptomi/aptomi/pkg/plugin"
	"github.com/Aptomi/aptomi/pkg/runtime"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

type dependencyResourcesWrapper struct {
	Resources plugin.Resources
}

func (g *dependencyResourcesWrapper) GetKind() string {
	return "dependencyResources"
}

func (api *coreAPI) handleDependencyResourcesGet(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	gen := runtime.LastGen
	policy, _, err := api.store.GetPolicy(gen)
	if err != nil {
		panic(fmt.Sprintf("error while getting requested policy: %s", err))
	}

	ns := params.ByName("ns")
	kind := lang.DependencyObject.Kind
	name := params.ByName("name")

	obj, err := policy.GetObject(kind, name, ns)
	if err != nil {
		panic(fmt.Sprintf("error while getting object %s/%s/%s in policy #%s", ns, kind, name, gen))
	}
	if obj == nil {
		api.contentType.WriteOneWithStatus(writer, request, nil, http.StatusNotFound)
	}

	// once dependency is loaded, we need to find its state in the actual state
	dependency := obj.(*lang.Dependency) // nolint: errcheck
	actualState, err := api.store.GetActualState()
	if err != nil {
		panic(fmt.Sprintf("Can't load actual state to get endpoints: %s", err))
	}

	plugins := api.pluginRegistryFactory()
	depKey := runtime.KeyForStorable(dependency)
	resources := make(plugin.Resources)
	rMergeMutex := sync.Mutex{}
	var wg sync.WaitGroup
	errors := make(chan error, 1)
	for _, instance := range actualState.ComponentInstanceMap {
		if _, ok := instance.DependencyKeys[depKey]; ok {
			// if component instance is not code, skip it
			if !instance.IsCode {
				continue
			}

			wg.Add(1)
			go func(instance *resolve.ComponentInstance) {
				// make sure we are converting panics into errors
				defer wg.Done()
				defer func() {
					if err := recover(); err != nil {
						select {
						case errors <- fmt.Errorf("panic: %s\n%s", err, string(debug.Stack())):
							// message sent
						default:
							// error was already there before, do nothing (but we have to keep an empty default block)
						}
					}
				}()

				codePlugin, pluginErr := pluginForComponentInstance(instance, policy, plugins)
				if pluginErr != nil {
					panic(fmt.Sprintf("Can't get plugin for component instance %s: %s", instance.GetKey(), pluginErr))
				}

				instanceResources, resErr := codePlugin.Resources(
					&plugin.CodePluginInvocationParams{
						DeployName:   instance.GetDeployName(),
						Params:       instance.CalculatedCodeParams,
						PluginParams: map[string]string{plugin.ParamTargetSuffix: instance.Metadata.Key.TargetSuffix},
						EventLog:     event.NewLog(logrus.WarnLevel, "resources"),
					},
				)

				if resErr != nil {
					panic(fmt.Sprintf("Error while getting deployment resources for component instance %s: %s", instance.GetKey(), resErr))
				}

				// merge resources
				rMergeMutex.Lock()
				defer rMergeMutex.Unlock()
				resources.Merge(instanceResources)

			}(instance)

		}
	}

	// wait until all go routines are over
	wg.Wait()

	// see if there were any errors
	select {
	case err := <-errors:
		panic(err)
	default:
		// no error, do nothing (but we have to keep an empty default block)
	}

	api.contentType.WriteOne(writer, request, &dependencyResourcesWrapper{Resources: resources})
}

func pluginForComponentInstance(instance *resolve.ComponentInstance, policy *lang.Policy, plugins plugin.Registry) (plugin.CodePlugin, error) {
	serviceObj, err := policy.GetObject(lang.ServiceObject.Kind, instance.Metadata.Key.ServiceName, instance.Metadata.Key.Namespace)
	if err != nil {
		return nil, err
	}
	component := serviceObj.(*lang.Service).GetComponentsMap()[instance.Metadata.Key.ComponentName]

	if component == nil || component.Code == nil {
		return nil, nil
	}

	clusterObj, err := policy.GetObject(lang.ClusterObject.Kind, instance.Metadata.Key.ClusterName, instance.Metadata.Key.ClusterNameSpace)
	if err != nil {
		return nil, err
	}
	if clusterObj == nil {
		return nil, fmt.Errorf("can't find cluster '%s/%s'", instance.Metadata.Key.ClusterNameSpace, instance.Metadata.Key.ClusterName)
	}
	cluster := clusterObj.(*lang.Cluster) // nolint: errcheck

	return plugins.ForCodeType(cluster, component.Code.Type)
}
