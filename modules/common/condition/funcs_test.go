/*
Copyright 2022 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +kubebuilder:object:generate:=true

package condition

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	unknownReady = UnknownCondition(ReadyCondition, RequestedReason, ReadyInitMessage)
	trueReady    = TrueCondition(ReadyCondition, ReadyMessage)

	unknownA    = UnknownCondition("a", "reason unknownA", "message unknownA")
	falseA      = FalseCondition("a", "reason falseA", SeverityInfo, "message falseA")
	trueA       = TrueCondition("a", "message trueA")
	unknownB    = UnknownCondition("b", "reason unknownB", "message unknownB")
	falseB      = FalseCondition("b", "reason falseB", SeverityInfo, "message falseB")
	falseBError = FalseCondition("b", "reason falseBError", SeverityError, "message falseBError")
	trueB       = TrueCondition("b", "message trueB")
	falseInfo   = FalseCondition("falseInfo", "reason falseInfo", SeverityInfo, "message falseInfo")
	falseError  = FalseCondition("falseError", "reason falseError", SeverityError, "message falseError")
)

func TestInit(t *testing.T) {
	tests := []struct {
		name       string
		conditions Conditions
		want       Conditions
	}{
		{
			name:       "Init conditions without optional condition",
			conditions: CreateList(nil),
			want:       CreateList(unknownReady),
		},
		{
			name:       "Init conditions with an optional condition",
			conditions: CreateList(unknownA),
			want:       CreateList(unknownReady, unknownA),
		},
		{
			name:       "Init conditions with optional conditions",
			conditions: CreateList(unknownA, unknownB),
			want:       CreateList(unknownReady, unknownA, unknownB),
		},
		{
			name:       "Init conditions with optional conditions",
			conditions: CreateList(unknownB, unknownA),
			want:       CreateList(unknownReady, unknownA, unknownB),
		},
		{
			name:       "Init conditions with duplicate optional condition",
			conditions: CreateList(unknownA, unknownA),
			want:       CreateList(unknownReady, unknownA),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			conditions := Conditions{}

			conditions.Init(&tt.conditions)
			g.Expect(conditions).To(haveSameConditionsOf(tt.want))
		})
	}
}

func TestSet(t *testing.T) {
	conditions := Conditions{}

	time1 := metav1.NewTime(time.Date(2022, time.August, 9, 10, 0, 0, 0, time.UTC))
	time2 := metav1.NewTime(time.Date(2022, time.August, 10, 10, 0, 0, 0, time.UTC))
	falseBTime1 := falseB.DeepCopy()
	falseBTime1.LastTransitionTime = time1

	falseBTime2 := falseB.DeepCopy()
	falseBTime2.LastTransitionTime = time2

	tests := []struct {
		name      string
		condition *Condition
		want      Conditions
	}{
		{
			name:      "Add nil condition",
			condition: nil,
			want:      CreateList(unknownReady),
		},
		{
			name:      "Add unknownB condition",
			condition: unknownB,
			want:      CreateList(unknownReady, unknownB),
		},
		{
			name:      "Add another condition unknownA, gets sorted",
			condition: unknownA,
			want:      CreateList(unknownReady, unknownA, unknownB),
		},
		{
			name:      "Add same condition unknownA, won't duplicate",
			condition: unknownA,
			want:      CreateList(unknownReady, unknownA, unknownB),
		},
		{
			name:      "Change condition unknownA, to be Status=False",
			condition: falseA,
			want:      CreateList(unknownReady, falseA, unknownB),
		},
		{
			name:      "Change ready condition to True",
			condition: trueReady,
			want:      CreateList(trueReady, falseA, unknownB),
		},
	}

	g := NewWithT(t)

	conditions.Init(nil)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady)))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			conditions.Set(tt.condition)

			g.Expect(conditions).To(haveSameConditionsOf(tt.want))
		})
	}

	// test time LastTransitionTime won't change if same status get set
	// a) set conditions with time1
	conditions.Set(falseBTime1)
	c1 := conditions.Get(falseB.Type)
	g.Expect(c1.LastTransitionTime).To(BeIdenticalTo(time1))

	// b) set condition with same state, but new time. Time must be still time1
	conditions.Set(falseBTime2)
	c2 := conditions.Get(falseB.Type)
	g.Expect(c2.LastTransitionTime).To(BeIdenticalTo(time1))
}

func TestHasSameState(t *testing.T) {
	g := NewWithT(t)

	// same condition
	falseInfo2 := falseInfo.DeepCopy()
	g.Expect(hasSameState(falseInfo, falseInfo2)).To(BeTrue())

	// different LastTransitionTime does not impact state
	falseInfo2 = falseInfo.DeepCopy()
	falseInfo2.LastTransitionTime = metav1.NewTime(time.Date(1900, time.November, 10, 23, 0, 0, 0, time.UTC))
	g.Expect(hasSameState(falseInfo, falseInfo2)).To(BeTrue())

	// different Type, Status, Reason, Severity and Message determine different state
	falseInfo2 = falseInfo.DeepCopy()
	falseInfo2.Type = "another type"
	g.Expect(hasSameState(falseInfo, falseInfo2)).To(BeFalse())

	falseInfo2 = falseInfo.DeepCopy()
	falseInfo2.Status = corev1.ConditionTrue
	g.Expect(hasSameState(falseInfo, falseInfo2)).To(BeFalse())

	falseInfo2 = falseInfo.DeepCopy()
	falseInfo2.Severity = SeverityWarning
	g.Expect(hasSameState(falseInfo, falseInfo2)).To(BeFalse())

	falseInfo2 = falseInfo.DeepCopy()
	falseInfo2.Reason = "another reason"
	g.Expect(hasSameState(falseInfo, falseInfo2)).To(BeFalse())

	falseInfo2 = falseInfo.DeepCopy()
	falseInfo2.Message = "another message"
	g.Expect(hasSameState(falseInfo, falseInfo2)).To(BeFalse())
}

func TestLess(t *testing.T) {
	g := NewWithT(t)

	// alphabetical order of Type is respected
	g.Expect(less(trueA, trueB)).To(BeTrue())
	g.Expect(less(trueB, trueA)).To(BeFalse())

	// Ready condition is always expected to be first
	g.Expect(less(trueReady, trueA)).To(BeTrue())
	g.Expect(less(trueA, trueReady)).To(BeFalse())

}

func TestGetAndHas(t *testing.T) {
	g := NewWithT(t)

	conditions := Conditions{}
	conditions.Init(nil)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady)))
	g.Expect(conditions.Has(ReadyCondition)).To(BeTrue())
	g.Expect(conditions.Get(ReadyCondition)).To(haveSameStateOf(unknownReady))
	g.Expect(conditions.Get("notExistingCond")).To(BeNil())
	g.Expect(conditions.Get("notExistingCond")).To(BeNil())

	conditions.Set(unknownA)
	g.Expect(conditions.Has(ReadyCondition)).To(BeTrue())
	g.Expect(conditions.Has("a")).To(BeTrue())
	g.Expect(conditions.Get("a")).To(haveSameStateOf(unknownA))
}

func TestIsMethods(t *testing.T) {
	g := NewWithT(t)

	conditions := Conditions{}
	cl := CreateList(trueA, falseInfo, unknownB)
	conditions.Init(&cl)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady, trueA, unknownB, falseInfo)))

	// test isTrue
	g.Expect(conditions.IsTrue("a")).To(BeTrue())
	g.Expect(conditions.IsTrue("falseInfo")).To(BeFalse())
	g.Expect(conditions.IsTrue("unknownB")).To(BeFalse())

	// test isFalse
	g.Expect(conditions.IsFalse("a")).To(BeFalse())
	g.Expect(conditions.IsFalse("falseInfo")).To(BeTrue())
	g.Expect(conditions.IsFalse("unknownB")).To(BeFalse())

	// test isUnknown
	g.Expect(conditions.IsUnknown("a")).To(BeFalse())
	g.Expect(conditions.IsUnknown("falseInfo")).To(BeFalse())
	g.Expect(conditions.IsUnknown("unknownB")).To(BeTrue())
}

func TestMarkMethods(t *testing.T) {
	g := NewWithT(t)

	conditions := Conditions{}
	conditions.Init(nil)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady)))
	g.Expect(conditions.Get(ReadyCondition).Severity).To(BeEmpty())

	// test MarkTrue
	conditions.MarkTrue(ReadyCondition, ReadyMessage)
	g.Expect(conditions.Get(ReadyCondition)).To(haveSameStateOf(trueReady))
	g.Expect(conditions.Get(ReadyCondition).Severity).To(BeEmpty())

	// test MarkFalse
	conditions.MarkFalse("falseError", "reason falseError", SeverityError, "message falseError")
	g.Expect(conditions.Get("falseError")).To(haveSameStateOf(falseError))

	// test MarkTrue of previous false condition
	conditions.MarkTrue("falseError", "now True")
	g.Expect(conditions.Get("falseError").Severity).To(BeEmpty())

	// test MarkUnknown
	conditions.MarkUnknown("a", "reason unknownA", "message unknownA")
	g.Expect(conditions.Get("a")).To(haveSameStateOf(unknownA))
}

func TestSortByLastTransitionTime(t *testing.T) {
	g := NewWithT(t)

	time1 := metav1.NewTime(time.Date(2020, time.August, 9, 10, 0, 0, 0, time.UTC))
	time2 := metav1.NewTime(time.Date(2020, time.August, 10, 10, 0, 0, 0, time.UTC))
	time3 := metav1.NewTime(time.Date(2020, time.August, 11, 10, 0, 0, 0, time.UTC))

	falseA.LastTransitionTime = time1
	falseB.LastTransitionTime = time2
	falseError.LastTransitionTime = time3

	g.Expect(lessLastTransitionTime(falseA, falseB)).To(BeFalse())
	g.Expect(lessLastTransitionTime(falseB, falseA)).To(BeTrue())

	conditions := Conditions{}
	cl := CreateList(falseB, falseError, falseA)
	conditions.Init(&cl)
	conditions.SortByLastTransitionTime()

	// unknownReady has the current time stamp, so is first
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady, falseError, falseB, falseA)))
}

func TestMirror(t *testing.T) {
	g := NewWithT(t)

	time1 := metav1.NewTime(time.Date(2020, time.August, 9, 10, 0, 0, 0, time.UTC))
	time2 := metav1.NewTime(time.Date(2020, time.August, 10, 10, 0, 0, 0, time.UTC))

	trueA.LastTransitionTime = time1
	falseB.LastTransitionTime = time2

	conditions := Conditions{}
	conditions.Init(nil)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady)))
	targetCondition := conditions.Mirror("targetConditon")
	g.Expect(targetCondition.Status).To(BeIdenticalTo(unknownReady.Status))
	g.Expect(targetCondition.Severity).To(BeIdenticalTo(unknownReady.Severity))
	g.Expect(targetCondition.Reason).To(BeIdenticalTo(unknownReady.Reason))
	g.Expect(targetCondition.Message).To(BeIdenticalTo(unknownReady.Message))

	conditions.Set(trueA)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady, trueA)))
	targetCondition = conditions.Mirror("targetConditon")
	// expect to be mirrored trueA
	g.Expect(targetCondition.Status).To(BeIdenticalTo(trueA.Status))
	g.Expect(targetCondition.Severity).To(BeIdenticalTo(trueA.Severity))
	g.Expect(targetCondition.Reason).To(BeIdenticalTo(trueA.Reason))
	g.Expect(targetCondition.Message).To(BeIdenticalTo(trueA.Message))

	conditions.Set(falseB)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady, trueA, falseB)))
	targetCondition = conditions.Mirror("targetConditon")
	// expect to be mirrored falseB
	g.Expect(targetCondition.Status).To(BeIdenticalTo(falseB.Status))
	g.Expect(targetCondition.Severity).To(BeIdenticalTo(falseB.Severity))
	g.Expect(targetCondition.Reason).To(BeIdenticalTo(falseB.Reason))
	g.Expect(targetCondition.Message).To(BeIdenticalTo(falseB.Message))

	conditions.Set(falseBError)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(unknownReady, trueA, falseBError)))
	targetCondition = conditions.Mirror("targetConditon")
	// expect to be mirrored falseBError
	g.Expect(targetCondition.Status).To(BeIdenticalTo(falseBError.Status))
	g.Expect(targetCondition.Severity).To(BeIdenticalTo(falseBError.Severity))
	g.Expect(targetCondition.Reason).To(BeIdenticalTo(falseBError.Reason))
	g.Expect(targetCondition.Message).To(BeIdenticalTo(falseBError.Message))

	// mark all non true conditions to be true
	conditions.MarkTrue(ReadyCondition, ReadyMessage)
	conditions.MarkTrue(trueB.Type, trueB.Message)
	g.Expect(conditions).To(haveSameConditionsOf(CreateList(trueReady, trueA, trueB)))
	targetCondition = conditions.Mirror("targetConditon")
	// expect to be mirrored trueReady
	g.Expect(targetCondition.Status).To(BeIdenticalTo(trueReady.Status))
	g.Expect(targetCondition.Severity).To(BeIdenticalTo(trueReady.Severity))
	g.Expect(targetCondition.Reason).To(BeIdenticalTo(trueReady.Reason))
	g.Expect(targetCondition.Message).To(BeIdenticalTo(trueReady.Message))
}

// haveSameConditionsOf matches a conditions list to be the same as another.
func haveSameConditionsOf(expected Conditions) types.GomegaMatcher {
	return &conditionsMatcher{
		Expected: expected,
	}
}

type conditionsMatcher struct {
	Expected Conditions
}

func (matcher *conditionsMatcher) Match(actual interface{}) (success bool, err error) {
	actualConditions, ok := actual.(Conditions)
	if !ok {
		return false, errors.New("Value should be a conditions list")
	}

	if len(actualConditions) != len(matcher.Expected) {
		return false, nil
	}

	for i := range actualConditions {
		if !hasSameState(&actualConditions[i], &matcher.Expected[i]) {
			return false, nil
		}
	}
	return true, nil
}

func (matcher *conditionsMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to have the same conditions of", matcher.Expected)
}

func (matcher *conditionsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to have the same conditions of", matcher.Expected)
}

// haveSameStateOf matches a condition to have the same state of another.
func haveSameStateOf(expected *Condition) types.GomegaMatcher {
	return &conditionMatcher{
		Expected: expected,
	}
}

type conditionMatcher struct {
	Expected *Condition
}

func (matcher *conditionMatcher) Match(actual interface{}) (success bool, err error) {
	actualCondition, ok := actual.(*Condition)
	if !ok {
		return false, errors.New("value should be a condition")
	}

	return hasSameState(actualCondition, matcher.Expected), nil
}

func (matcher *conditionMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to have the same state of", matcher.Expected)
}

func (matcher *conditionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to have the same state of", matcher.Expected)
}