package v2

import (
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator"
)

func ToTranslatorInfraConnection(c v2.WBInfraConnection) translator.InfraConnection {
	return translator.InfraConnection{
		URL: c.URL,
	}
}

func ToWbInfraConnection(c translator.InfraConnection) v2.WBInfraConnection {
	return v2.WBInfraConnection{
		URL: c.URL,
	}
}

func ToTranslatorInfraStatus(s v2.WBInfraStatus) translator.InfraStatus {
	return translator.InfraStatus{
		Ready:      s.Ready,
		State:      s.State,
		Conditions: s.Conditions,
		Connection: translator.InfraConnection{
			URL: s.Connection.URL,
		},
	}
}

func ToWbInfraStatus(s translator.InfraStatus) v2.WBInfraStatus {
	return v2.WBInfraStatus{
		Ready:      s.Ready,
		State:      s.State,
		Conditions: s.Conditions,
		Connection: v2.WBInfraConnection{
			URL: s.Connection.URL,
		},
	}
}
