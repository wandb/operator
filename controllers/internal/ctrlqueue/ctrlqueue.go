package ctrlqueue

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	defaultRequeueDelay = 10 * time.Second
)

func DoNotRequeue() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func Requeue(err error) (ctrl.Result, error) {
	return ctrl.Result{}, err
}

func RequeueWithDelay() (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: defaultRequeueDelay}, nil
}
