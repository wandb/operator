package ctrlqueue

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	CheckForUpdatesFrequency = time.Hour * 2
)

func DoNotRequeue() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func Requeue(err error) (ctrl.Result, error) {
	return ctrl.Result{}, err
}

func RequeueWithDelay(delay time.Duration) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: delay}, nil
}

func ContainsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}

	return false
}
