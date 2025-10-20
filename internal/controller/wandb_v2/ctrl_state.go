package wandb_v2

import (
	"errors"

	ctrl "sigs.k8s.io/controller-runtime"
)

type CtrlState interface {
	isDone() bool
	reconcileResult() (ctrl.Result, error)
}

func CtrlError(err error) CtrlState {
	return &ctrlStateImpl{
		done:   true,
		err:    err,
		result: ctrl.Result{},
	}
}

func CtrlContinue() CtrlState {
	return &ctrlStateImpl{
		done: false,
	}
}

func CtrlDone(result ctrl.Result) CtrlState {
	return &ctrlStateImpl{
		done:   true,
		result: result,
	}
}

type ctrlStateImpl struct {
	done   bool
	err    error
	result ctrl.Result
}

func (d *ctrlStateImpl) isDone() bool {
	return d.done
}

func (d *ctrlStateImpl) reconcileResult() (ctrl.Result, error) {
	if !d.isDone() {
		return ctrl.Result{}, errors.New("returning undone reconcile result")
	}
	return d.result, d.err
}
