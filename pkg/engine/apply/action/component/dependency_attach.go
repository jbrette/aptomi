package component

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/Aptomi/aptomi/pkg/engine/apply/action"
	"github.com/Aptomi/aptomi/pkg/engine/resolve"
	"github.com/Aptomi/aptomi/pkg/runtime"
	"github.com/Aptomi/aptomi/pkg/util"
)

// AttachDependencyActionObject is an informational data structure with Kind and Constructor for the action
var AttachDependencyActionObject = &runtime.Info{
	Kind:        "action-component-dependency-attach",
	Constructor: func() runtime.Object { return &AttachDependencyAction{} },
}

// AttachDependencyAction is a action which gets called when a consumer is added to an existing component
type AttachDependencyAction struct {
	runtime.TypeKind `yaml:",inline"`
	*action.Metadata
	ComponentKey string
	DependencyID string
	Depth        int
}

// NewAttachDependencyAction creates new AttachDependencyAction
func NewAttachDependencyAction(componentKey string, dependencyID string, depth int) *AttachDependencyAction {
	return &AttachDependencyAction{
		TypeKind:     AttachDependencyActionObject.GetTypeKind(),
		Metadata:     action.NewMetadata(AttachDependencyActionObject.Kind, componentKey, dependencyID),
		ComponentKey: componentKey,
		DependencyID: dependencyID,
		Depth:        depth,
	}
}

// Apply applies the action
func (a *AttachDependencyAction) Apply(context *action.Context) (errResult error) {
	start := time.Now()
	defer func() {
		if err := recover(); err != nil {
			errResult = fmt.Errorf("panic: %s\n%s", err, string(debug.Stack()))
		}

		action.CollectMetricsFor(a, start, errResult)
	}()

	context.EventLog.NewEntry().Debugf("Attaching dependency '%s' to component instance: '%s'", a.DependencyID, a.ComponentKey)

	return context.ActualStateUpdater.UpdateComponentInstance(a.ComponentKey, func(obj *resolve.ComponentInstance) {
		obj.DependencyKeys[a.DependencyID] = a.Depth
	})
}

// DescribeChanges returns text-based description of changes that will be applied
func (a *AttachDependencyAction) DescribeChanges() util.NestedParameterMap {
	return util.NestedParameterMap{
		"kind":       a.Kind,
		"key":        a.ComponentKey,
		"dependency": a.DependencyID,
		"pretty":     fmt.Sprintf("[>] %s = %s", a.ComponentKey, a.DependencyID),
	}
}
