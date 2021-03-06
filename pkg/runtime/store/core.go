package store

import (
	"github.com/Aptomi/aptomi/pkg/engine"
	"github.com/Aptomi/aptomi/pkg/engine/actual"
	"github.com/Aptomi/aptomi/pkg/engine/apply/action"
	"github.com/Aptomi/aptomi/pkg/engine/resolve"
	"github.com/Aptomi/aptomi/pkg/lang"
	"github.com/Aptomi/aptomi/pkg/runtime"
)

// Core represents main object store interface that covers database operations for all objects
type Core interface {
	Policy
	Revision
	ActualState
}

// Policy represents database operations for Policy object
type Policy interface {
	GetPolicy(runtime.Generation) (*lang.Policy, runtime.Generation, error)
	GetPolicyData(runtime.Generation) (*engine.PolicyData, error)
	InitPolicy() error
	UpdatePolicy(updated []lang.Base, performedBy string) (changed bool, data *engine.PolicyData, err error)
	DeleteFromPolicy(deleted []lang.Base, performedBy string) (changed bool, data *engine.PolicyData, err error)
}

// Revision represents database operations for Revision object
type Revision interface {
	NewRevision(policyGen runtime.Generation, desiredState *resolve.PolicyResolution, recalculateAll bool) (*engine.Revision, error)
	GetDesiredState(*engine.Revision) (*resolve.PolicyResolution, error)
	GetRevision(gen runtime.Generation) (*engine.Revision, error)
	UpdateRevision(revision *engine.Revision) error
	NewRevisionResultUpdater(revision *engine.Revision) action.ApplyResultUpdater
	GetFirstUnprocessedRevision() (*engine.Revision, error)
	GetLastRevisionForPolicy(policyGen runtime.Generation) (*engine.Revision, error)
	GetAllRevisionsForPolicy(policyGen runtime.Generation) ([]*engine.Revision, error)
}

// ActualState represents database operations for the actual state handling
type ActualState interface {
	GetActualState() (*resolve.PolicyResolution, error)
	NewActualStateUpdater(*resolve.PolicyResolution) actual.StateUpdater
}
