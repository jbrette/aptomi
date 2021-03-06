package visualization

import (
	"github.com/Aptomi/aptomi/pkg/engine/resolve"
	"github.com/Aptomi/aptomi/pkg/lang"
	"github.com/Aptomi/aptomi/pkg/runtime"
)

// Object produces a graph which represents an object
func (b *GraphBuilder) Object(obj runtime.Object) *Graph {
	if service, ok := obj.(*lang.Service); ok {
		b.traceService(service, nil, "", 0, PolicyCfgDefault)
	}
	if contract, ok := obj.(*lang.Contract); ok {
		b.traceContract(contract, nil, "", 0, PolicyCfgDefault)
	}
	if dependency, ok := obj.(*lang.Dependency); ok {
		b.traceDependencyResolution("", dependency, nil, 0, DependencyResolutionCfgDefault, func(*resolve.ComponentInstance) bool { return true })
	}
	return b.graph
}

func (b *GraphBuilder) traceContract(contract *lang.Contract, last graphNode, lastLabel string, level int, cfg *PolicyCfg) {
	// [last] -> contract
	ctrNode := contractNode{contract: contract}
	b.graph.addNode(ctrNode, level)
	if last != nil {
		b.graph.addEdge(newEdge(last, ctrNode, lastLabel))
	}

	// show all contexts within a given contract
	for _, context := range contract.Contexts {
		// contract -> [context] as edge label -> service
		// lookup the corresponding service
		serviceObj, errService := b.policy.GetObject(lang.ServiceObject.Kind, context.Allocation.Service, contract.Namespace)
		if errService != nil {
			b.graph.addNode(errorNode{err: errService}, level)
			continue
		}
		service := serviceObj.(*lang.Service) // nolint: errcheck

		// context -> service
		contextName := context.Name
		if len(context.Allocation.Keys) > 0 {
			contextName += " (+)"
		}
		b.traceService(service, ctrNode, contextName, level+1, cfg)
	}
}

func (b *GraphBuilder) traceService(service *lang.Service, last graphNode, lastLabel string, level int, cfg *PolicyCfg) {
	svcNode := serviceNode{service: service}
	b.graph.addNode(svcNode, level)
	if last != nil {
		b.graph.addEdge(newEdge(last, svcNode, lastLabel))
	}

	// process components first
	showedComponents := false
	for _, component := range service.Components {
		if component.Code != nil && cfg.showServiceComponents {
			// service -> component
			cmpNode := componentNode{service: service, component: component}
			b.graph.addNode(cmpNode, level+1)
			b.graph.addEdge(newEdge(svcNode, cmpNode, ""))
			showedComponents = true
		}
	}

	// do not show any more service components down the tree if we already showed them at top level
	cfgNext := &PolicyCfg{}
	*cfgNext = *cfg
	if cfg.showServiceComponentsOnlyForTopLevel && showedComponents {
		cfgNext.showServiceComponents = false
	}

	// process contracts after that
	for _, component := range service.Components {
		if len(component.Contract) > 0 {
			contractObjNew, errContract := b.policy.GetObject(lang.ContractObject.Kind, component.Contract, service.Namespace)
			if errContract != nil {
				b.graph.addNode(errorNode{err: errContract}, level+1)
				continue
			}
			b.traceContract(contractObjNew.(*lang.Contract), svcNode, "", level+1, cfgNext)
		}
	}

}
