package common

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

// CtrlScope defines the scope of the controller operation for determining
// that level of the reconciliation loop is complete.
// A larger numeric scope value indicates a broader scope.
type CtrlScope int

const (
	NoScope         CtrlScope = 0 // an `exitScope` of NoScope is simply to continue
	PackageScope    CtrlScope = 1
	ReconcilerScope CtrlScope = 2
)

type CtrlState interface {
	ShouldExit(scope CtrlScope) bool
	ReconcilerResult() (ctrl.Result, error)
}

func CtrlError(err error) CtrlState {
	return &ctrlStateImpl{
		exitScope: ReconcilerScope,
		err:       err,
		result:    ctrl.Result{},
	}
}

func CtrlContinue() CtrlState {
	return CtrlDone(NoScope)
}

func CtrlDoneUntil(scope CtrlScope, requeueAfter time.Duration) CtrlState {
	return &ctrlStateImpl{
		exitScope: scope,
		result:    ctrl.Result{RequeueAfter: requeueAfter},
	}
}

func CtrlDone(scope CtrlScope) CtrlState {
	return &ctrlStateImpl{
		exitScope: scope,
		result:    ctrl.Result{},
	}
}

type ctrlStateImpl struct {
	exitScope CtrlScope
	err       error
	result    ctrl.Result
}

func (d *ctrlStateImpl) ShouldExit(scope CtrlScope) bool {
	return d.exitScope >= scope
}

func (d *ctrlStateImpl) ReconcilerResult() (ctrl.Result, error) {
	return d.result, d.err
}
