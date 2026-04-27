package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func cleanupNetworkingModeResources(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	switch wandb.Spec.Networking.Mode {
	case apiv2.NetworkingModeIngress:
		if err := deleteGateway(ctx, c, wandb); err != nil {
			return err
		}
		if err := deleteInfraHTTPRoutes(ctx, c, wandb); err != nil {
			return err
		}
	case apiv2.NetworkingModeGatewayAPI:
		if err := deleteConsolidatedIngress(ctx, c, wandb); err != nil {
			return err
		}
		if wandb.Spec.Networking.GatewayAPI == nil || !wandb.Spec.Networking.GatewayAPI.Gateway.Managed {
			if err := deleteGateway(ctx, c, wandb); err != nil {
				return err
			}
		}
	case apiv2.NetworkingModeNone:
		if err := deleteConsolidatedIngress(ctx, c, wandb); err != nil {
			return err
		}
		if err := deleteGateway(ctx, c, wandb); err != nil {
			return err
		}
		if err := deleteInfraHTTPRoutes(ctx, c, wandb); err != nil {
			return err
		}
	}

	return nil
}

func resetInactiveNetworkingStatus(wandb *apiv2.WeightsAndBiases) {
	switch wandb.Spec.Networking.Mode {
	case apiv2.NetworkingModeIngress:
		wandb.Status.GatewayStatus = nil
	case apiv2.NetworkingModeGatewayAPI:
		wandb.Status.IngressStatus = nil
	case apiv2.NetworkingModeNone:
		wandb.Status.GatewayStatus = nil
		wandb.Status.IngressStatus = nil
	}
}
