package model

import (
	"errors"
	"fmt"
)

type InfraError struct {
	infraName infraName
	code      string
	reason    string
}

func (e InfraError) Error() string {
	return fmt.Sprintf("%s(%s): %s", e.code, e.infraName, e.reason)
}

func IsInfraError(err error, infraNames ...infraName) bool {
	var infraError InfraError
	ok := errors.As(err, &infraError)
	if len(infraNames) == 0 {
		return ok
	}
	if ok {
		for _, n := range infraNames {
			if n == infraError.infraName {
				return true
			}
		}
		return false
	}
	return false
}

// HasCriticalError will inform the reconciler if there is an error that is
// not an InfraError; if so, this is considered a critical error for the
// Controller/Reconciler
func HasCriticalError(errorList []error) bool {
	for _, e := range errorList {
		if !IsInfraError(e) {
			return true
		}
	}
	return false
}

func IsCriticalError(err error) bool {
	return !IsInfraError(err)
}

func ToInfraError(err error) (InfraError, bool) {
	var infraError InfraError
	ok := errors.As(err, &infraError)
	if ok {
		return infraError, true
	}
	return InfraError{}, false
}

func ToRedisInfraError(err error) (RedisInfraError, bool) {
	var infraError InfraError
	var ok bool
	infraError, ok = ToInfraError(err)
	if !ok {
		return RedisInfraError{}, false
	}
	result := RedisInfraError{}
	if infraError.infraName != Redis {
		return result, false
	}
	result.infraName = infraError.infraName
	result.code = infraError.code
	result.reason = infraError.reason
	return result, true
}
