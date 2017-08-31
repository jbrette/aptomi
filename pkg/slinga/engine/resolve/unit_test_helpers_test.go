package resolve

import (
	"github.com/Aptomi/aptomi/pkg/slinga/eventlog"
	. "github.com/Aptomi/aptomi/pkg/slinga/language"
	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

const (
	ResSuccess = iota
	ResError   = iota
)

type UnitTestLogVerifier struct {
	checkForErrorMessage string
	present              bool
}

func NewUnitTestLogVerifier(checkForErrorMessage string) *UnitTestLogVerifier {
	return &UnitTestLogVerifier{checkForErrorMessage: checkForErrorMessage}
}

func (verifier *UnitTestLogVerifier) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (verifier *UnitTestLogVerifier) Fire(e *logrus.Entry) error {
	if e.Level == logrus.ErrorLevel && strings.Contains(e.Message, verifier.checkForErrorMessage) {
		verifier.present = true
	}
	return nil
}

func loadUnitTestsPolicy() *PolicyNamespace {
	return LoadUnitTestsPolicy("../../testdata/unittests")
}

func loadPolicyAndResolve(t *testing.T) (*PolicyNamespace, *PolicyResolution) {
	policy := loadUnitTestsPolicy()
	return policy, resolvePolicy(t, policy, ResSuccess, "")
}

func resolvePolicy(t *testing.T, policy *PolicyNamespace, expectedResult int, expectedErrorMessage string) *PolicyResolution {
	userLoader := NewUserLoaderFromDir("../../testdata/unittests")
	resolver := NewPolicyResolver(policy, userLoader)
	result, err := resolver.ResolveAllDependencies()

	if !assert.Equal(t, expectedResult != ResError, err == nil, "Policy resolution status (success vs. error)") {
		// print log into stdout and exit
		hook := &eventlog.HookStdout{}
		resolver.eventLog.Save(hook)
		t.FailNow()
		return nil
	}

	if expectedResult == ResError {
		// check for error message
		verifier := NewUnitTestLogVerifier(expectedErrorMessage)
		resolver.eventLog.Save(verifier)
		if !assert.True(t, verifier.present, "Event log should have an error message containing words: "+expectedErrorMessage) {
			hook := &eventlog.HookStdout{}
			resolver.eventLog.Save(hook)
			t.FailNow()
		}
		return nil
	}

	return result.Resolution
}

func getInstanceInternal(t *testing.T, key string, resolutionData *ResolutionData) *ComponentInstance {
	instance, ok := resolutionData.ComponentInstanceMap[key]
	if !assert.True(t, ok, "Component instance in resolution data: "+key) {
		t.FailNow()
	}
	return instance
}

func getInstanceByParams(t *testing.T, serviceName string, contextName string, allocationKeysResolved []string, componentName string, policy *PolicyNamespace, resolution *PolicyResolution) *ComponentInstance {
	key := NewComponentInstanceKey(serviceName, policy.Contexts[contextName], allocationKeysResolved, policy.Services[serviceName].GetComponentsMap()[componentName])
	return getInstanceInternal(t, key.GetKey(), resolution.Resolved)
}
