package state_test

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/state"
	"github.com/wandb/operator/pkg/wandb/spec/state/statefakes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Versioning", func() {
	Describe("UserSpecName", func() {
		It("should return the expected name", func() {
			Expect(state.UserSpecName("foo")).To(Equal("foo-spec-user"))
		})
	})
	Describe("ActiveSpecName", func() {
		It("should return the expected name", func() {
			Expect(state.ActiveSpecName("foo")).To(Equal("foo-spec-active"))
		})
	})
	Describe("New", func() {
		It("should return a new Manager", func() {
			m := state.New(nil, nil, nil, nil, nil)
			Expect(m).To(BeAssignableToTypeOf(&state.Manager{}))
		})
	})
	Describe("Manager", func() {
		var m *state.Manager
		var mockState *statefakes.FakeState
		BeforeEach(func() {
			mockState = &statefakes.FakeState{}
			owner := &metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			}
			m = state.New(
				context.Background(),
				nil,
				owner,
				nil,
				mockState,
			)
		})
		Describe("SetUserInput", func() {
			It("set the user spec to the correct key", func() {
				inputSpec := &spec.Spec{}
				Expect(m.SetUserInput(inputSpec)).To(BeNil())
				Expect(mockState.SetCallCount()).To(Equal(1))
				fmt.Println(mockState.SetArgsForCall(0))
				arg1, arg2, arg3 := mockState.SetArgsForCall(0)
				Expect(arg1).To(Equal("bar"))
				Expect(arg2).To(Equal("foo-spec-user"))
				Expect(arg3).To(Equal(inputSpec))
			})
		})
		Describe("GetUserInput", func() {
			It("should get the user spec from the correct key", func() {
				_, err := m.GetUserInput()
				Expect(err).To(BeNil())
				Expect(mockState.GetCallCount()).To(Equal(1))
				arg1, arg2 := mockState.GetArgsForCall(0)
				Expect(arg1).To(Equal("bar"))
				Expect(arg2).To(Equal("foo-spec-user"))
			})
		})
		Describe("GetActive", func() {
			It("should get the active spec from the correct key", func() {
				_, err := m.GetActive()
				Expect(err).To(BeNil())
				Expect(mockState.GetCallCount()).To(Equal(1))
				arg1, arg2 := mockState.GetArgsForCall(0)
				Expect(arg1).To(Equal("bar"))
				Expect(arg2).To(Equal("foo-spec-active"))
			})
		})
		Describe("SetActive", func() {
			It("set the active spec to the correct key", func() {
				inputSpec := &spec.Spec{}
				Expect(m.SetActive(inputSpec)).To(BeNil())
				Expect(mockState.SetCallCount()).To(Equal(1))
				arg1, arg2, arg3 := mockState.SetArgsForCall(0)
				Expect(arg1).To(Equal("bar"))
				Expect(arg2).To(Equal("foo-spec-active"))
				Expect(arg3).To(Equal(inputSpec))
			})
		})
		Describe("Get", func() {
			It("should get the spec from the correct key", func() {
				_, err := m.Get("test-release")
				Expect(err).To(BeNil())
				Expect(mockState.GetCallCount()).To(Equal(1))
				arg1, arg2 := mockState.GetArgsForCall(0)
				Expect(arg1).To(Equal("bar"))
				Expect(arg2).To(Equal("foo-test-release"))
			})
		})
		Describe("Set", func() {
			It("set the spec to the correct key", func() {
				inputSpec := &spec.Spec{}
				Expect(m.Set("test-release", inputSpec)).To(BeNil())
				Expect(mockState.SetCallCount()).To(Equal(1))
				arg1, arg2, arg3 := mockState.SetArgsForCall(0)
				Expect(arg1).To(Equal("bar"))
				Expect(arg2).To(Equal("foo-test-release"))
				Expect(arg3).To(Equal(inputSpec))
			})
		})
	})
})
