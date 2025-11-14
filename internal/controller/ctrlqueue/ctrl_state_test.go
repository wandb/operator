package ctrlqueue

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("CtrlState", func() {
	Describe("CtrlError", func() {
		It("should create a state with error and ReconcilerScope", func() {
			err := errors.New("test error")
			state := CtrlError(err)

			Expect(state).ToNot(BeNil())
			Expect(state.ShouldExit(NoScope)).To(BeTrue())
			Expect(state.ShouldExit(PackageScope)).To(BeTrue())
			Expect(state.ShouldExit(ReconcilerScope)).To(BeTrue())

			result, returnedErr := state.ReconcilerResult()
			Expect(returnedErr).To(Equal(err))
			Expect(result).To(Equal(ctrl.Result{}))
		})

		It("should handle nil error", func() {
			state := CtrlError(nil)

			Expect(state).ToNot(BeNil())
			result, err := state.ReconcilerResult()
			Expect(err).To(BeNil())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	Describe("CtrlContinue", func() {
		It("should create a state with NoScope that only exits at NoScope level", func() {
			state := CtrlContinue()

			Expect(state).ToNot(BeNil())
			Expect(state.ShouldExit(NoScope)).To(BeTrue())
			Expect(state.ShouldExit(PackageScope)).To(BeFalse())
			Expect(state.ShouldExit(ReconcilerScope)).To(BeFalse())

			result, err := state.ReconcilerResult()
			Expect(err).To(BeNil())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	Describe("CtrlDone", func() {
		Context("when scope is NoScope", func() {
			It("should exit at NoScope level only", func() {
				state := CtrlDone(NoScope)

				Expect(state).ToNot(BeNil())
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeFalse())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeFalse())

				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		Context("when scope is PackageScope", func() {
			It("should exit at PackageScope and higher", func() {
				state := CtrlDone(PackageScope)

				Expect(state).ToNot(BeNil())
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeTrue())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeFalse())

				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		Context("when scope is ReconcilerScope", func() {
			It("should exit at all scopes", func() {
				state := CtrlDone(ReconcilerScope)

				Expect(state).ToNot(BeNil())
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeTrue())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeTrue())

				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})
	})

	Describe("CtrlDoneUntil", func() {
		Context("when scope is NoScope", func() {
			It("should exit at NoScope level only with requeue duration", func() {
				duration := 5 * time.Minute
				state := CtrlDoneUntil(NoScope, duration)

				Expect(state).ToNot(BeNil())
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeFalse())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeFalse())

				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result.RequeueAfter).To(Equal(duration))
			})
		})

		Context("when scope is PackageScope", func() {
			It("should exit at PackageScope and higher with requeue", func() {
				duration := 10 * time.Second
				state := CtrlDoneUntil(PackageScope, duration)

				Expect(state).ToNot(BeNil())
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeTrue())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeFalse())

				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result.RequeueAfter).To(Equal(duration))
			})
		})

		Context("when scope is ReconcilerScope", func() {
			It("should exit at all scopes with requeue", func() {
				duration := 1 * time.Hour
				state := CtrlDoneUntil(ReconcilerScope, duration)

				Expect(state).ToNot(BeNil())
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeTrue())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeTrue())

				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result.RequeueAfter).To(Equal(duration))
			})
		})

		Context("when duration is zero", func() {
			It("should handle zero duration", func() {
				state := CtrlDoneUntil(PackageScope, 0)

				Expect(state).ToNot(BeNil())
				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("when duration is negative", func() {
			It("should handle negative duration", func() {
				duration := -5 * time.Second
				state := CtrlDoneUntil(ReconcilerScope, duration)

				Expect(state).ToNot(BeNil())
				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result.RequeueAfter).To(Equal(duration))
			})
		})
	})

	Describe("ShouldExit", func() {
		Context("scope comparison logic", func() {
			It("should correctly compare NoScope exit state", func() {
				state := CtrlDone(NoScope)
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeFalse())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeFalse())
			})

			It("should correctly compare PackageScope exit state", func() {
				state := CtrlDone(PackageScope)
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeTrue())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeFalse())
			})

			It("should correctly compare ReconcilerScope exit state", func() {
				state := CtrlDone(ReconcilerScope)
				Expect(state.ShouldExit(NoScope)).To(BeTrue())
				Expect(state.ShouldExit(PackageScope)).To(BeTrue())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeTrue())
			})
		})
	})

	Describe("ReconcilerResult", func() {
		It("should return empty result and nil error for CtrlContinue", func() {
			state := CtrlContinue()
			result, err := state.ReconcilerResult()
			Expect(err).To(BeNil())
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(result.Requeue).To(BeFalse())
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})

		It("should return result with requeue duration", func() {
			duration := 30 * time.Second
			state := CtrlDoneUntil(PackageScope, duration)
			result, err := state.ReconcilerResult()
			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(Equal(duration))
		})

		It("should return error when present", func() {
			expectedErr := errors.New("reconciliation error")
			state := CtrlError(expectedErr)
			result, err := state.ReconcilerResult()
			Expect(err).To(Equal(expectedErr))
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	Describe("Scope constants", func() {
		It("should have correct numeric ordering", func() {
			Expect(int(NoScope)).To(Equal(0))
			Expect(int(PackageScope)).To(Equal(1))
			Expect(int(ReconcilerScope)).To(Equal(2))
			Expect(NoScope < PackageScope).To(BeTrue())
			Expect(PackageScope < ReconcilerScope).To(BeTrue())
		})
	})

	Describe("Integration scenarios", func() {
		Context("when handler needs to continue", func() {
			It("should allow reconciler to continue processing", func() {
				state := CtrlContinue()
				Expect(state.ShouldExit(PackageScope)).To(BeFalse())
			})
		})

		Context("when handler is done", func() {
			It("should exit handler but allow reconciler to continue", func() {
				state := CtrlDone(PackageScope)
				Expect(state.ShouldExit(PackageScope)).To(BeTrue())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeFalse())
			})
		})

		Context("when reconciler encounters error", func() {
			It("should exit all scopes", func() {
				state := CtrlError(errors.New("fatal error"))
				Expect(state.ShouldExit(PackageScope)).To(BeTrue())
				Expect(state.ShouldExit(ReconcilerScope)).To(BeTrue())
			})
		})

		Context("when reconciler needs requeue", func() {
			It("should exit all scopes with requeue duration", func() {
				duration := 2 * time.Minute
				state := CtrlDoneUntil(ReconcilerScope, duration)

				Expect(state.ShouldExit(ReconcilerScope)).To(BeTrue())
				result, err := state.ReconcilerResult()
				Expect(err).To(BeNil())
				Expect(result.RequeueAfter).To(Equal(duration))
			})
		})
	})
})
