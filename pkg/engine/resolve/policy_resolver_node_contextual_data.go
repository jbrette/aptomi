package resolve

import (
	"github.com/Aptomi/aptomi/pkg/lang"
	"github.com/Aptomi/aptomi/pkg/lang/expression"
	"github.com/Aptomi/aptomi/pkg/lang/template"
	"github.com/Aptomi/aptomi/pkg/runtime"
	"github.com/Aptomi/aptomi/pkg/util"
)

/*
	Data exposed to context expressions/criteria defined in policy
*/

// This method defines which contextual information will be exposed to the expression engine (for evaluating criteria)
// Be careful about what gets exposed through this method. User can refer to structs and their methods from the policy
func (node *resolutionNode) getContextualDataForContextExpression() *expression.Parameters {
	return expression.NewParams(
		node.labels.Labels,
		map[string]interface{}{},
	)
}

// This method defines which contextual information will be exposed to the expression engine (for evaluating criteria)
// Be careful about what gets exposed through this method. User can refer to structs and their methods from the policy
func (node *resolutionNode) getContextualDataForComponentCriteria() *expression.Parameters {
	return expression.NewParams(
		node.labels.Labels,
		map[string]interface{}{},
	)
}

/*
	Data exposed to rules defined
*/

// This method defines which contextual information will be exposed to the expression engine (for evaluating rules)
// Be careful about what gets exposed through this method. User can refer to structs and their methods from the policy
func (node *resolutionNode) getContextualDataForRuleExpression() *expression.Parameters {
	return expression.NewParams(
		node.labels.Labels,
		map[string]interface{}{
			"Service":    node.proxyService(node.service),
			"Dependency": node.proxyDependency(node.dependency),
		},
	)
}

/*
	Data exposed to templates defined in policy
*/

// This method defines which contextual information will be exposed to the template engine (for evaluating all templates - discovery, code params, etc)
// Be careful about what gets exposed through this method. User can refer to structs and their methods from the policy
func (node *resolutionNode) getContextualDataForContextAllocationTemplate() *template.Parameters {
	return template.NewParams(
		struct {
			User       interface{}
			Dependency interface{}
			Labels     interface{}
		}{
			User:       node.proxyUser(node.user),
			Dependency: node.proxyDependency(node.dependency),
			Labels:     node.labels.Labels,
		},
	)
}

// This method defines which contextual information will be exposed to the template engine (for evaluating all templates - discovery, code params, etc)
// Be careful about what gets exposed through this method. User can refer to structs and their methods from the policy
func (node *resolutionNode) getContextualDataForCodeDiscoveryTemplate() *template.Parameters {
	return template.NewParams(
		struct {
			User      interface{}
			Labels    interface{}
			Discovery interface{}
			Target    interface{}
		}{
			User:      node.proxyUser(node.user),
			Labels:    node.labels.Labels,
			Discovery: node.proxyDiscovery(node.discoveryTreeNode, node.componentKey),
			Target:    node.proxyTarget(node.componentKey),
		},
	)
}

/*
	Proxy functions
*/

// How service is visible from the policy language
func (node *resolutionNode) proxyService(service *lang.Service) interface{} {
	return struct {
		lang.Metadata
		Labels interface{}
	}{
		Metadata: service.Metadata,
		Labels:   service.Labels,
	}
}

// How user is visible from the policy language
func (node *resolutionNode) proxyUser(user *lang.User) interface{} {
	return struct {
		Name    interface{}
		Labels  interface{}
		Secrets interface{}
	}{
		Name:    user.Name,
		Labels:  user.Labels,
		Secrets: node.resolver.externalData.SecretLoader.LoadSecretsByUserName(user.Name),
	}
}

// How dependency is visible from the policy language
func (node *resolutionNode) proxyDependency(dependency *lang.Dependency) interface{} {
	result := struct {
		lang.Metadata
		ID interface{}
	}{
		Metadata: dependency.Metadata,
		ID:       runtime.KeyForStorable(dependency),
	}
	return result
}

// How target is visible from the policy language
func (node *resolutionNode) proxyTarget(cik *ComponentInstanceKey) interface{} {
	result := struct {
		Namespace string
	}{
		Namespace: cik.TargetSuffix,
	}
	return result
}

// How discovery tree is visible from the policy language
func (node *resolutionNode) proxyDiscovery(discoveryTree util.NestedParameterMap, cik *ComponentInstanceKey) interface{} {
	result := discoveryTree.MakeCopy()

	// special case to announce own component instance
	result["Instance"] = util.EscapeName(cik.GetDeployName())

	// special case to announce own component ID
	result["InstanceId"] = util.HashFnv(cik.GetKey())

	// expose parent service information as well
	if cik.IsComponent() {
		serviceCik := cik.GetParentServiceKey()
		result["Service"] = util.NestedParameterMap{
			// announce instance of the enclosing service
			"Instance": util.EscapeName(serviceCik.GetDeployName()),
			// announce instance of the enclosing service instance ID
			"InstanceId": util.HashFnv(serviceCik.GetKey()),
		}
	}

	return result
}
