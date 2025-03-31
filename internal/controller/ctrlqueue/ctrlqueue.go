package ctrlqueue

import (
	"fmt"
	"time"

	"github.com/wandb/operator/pkg/wandb/spec"
	ctrl "sigs.k8s.io/controller-runtime"
)

func DoNotRequeue() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func RequeueWithError(err error) (ctrl.Result, error) {
	return ctrl.Result{}, err
}

func Requeue(s *spec.Spec) (ctrl.Result, error) {
	delay := 1 * time.Hour
	reconcileFrequency := s.Values.GetString("reconcileFrequency")
	if reconcileFrequency != "" {
		parsedDelay, err := time.ParseDuration(reconcileFrequency)
		if err == nil {
			delay = parsedDelay
		} else {
			fmt.Println("error parsing reconcileFrequency", err)
		}
	}
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
