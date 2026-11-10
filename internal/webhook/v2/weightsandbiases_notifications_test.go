package v2

import (
	"strings"
	"testing"

	appsv2 "github.com/wandb/operator/api/v2"
)

func notificationSelector(name, key string) appsv2.ValueOrSecret {
	return appsv2.ValueFromSecret(name, key, false)
}

func TestValidateNotificationSpec(t *testing.T) {
	sink := notificationSelector("email", "sink")
	username := notificationSelector("smtp", "username")
	password := notificationSelector("smtp", "password")
	validSMTP := func() *appsv2.EmailSMTPSpec {
		return &appsv2.EmailSMTPSpec{
			Host:     notificationSelector("smtp", "host"),
			Port:     notificationSelector("smtp", "port"),
			Username: username,
			Password: password,
		}
	}

	cases := []struct {
		name    string
		mutate  func(*appsv2.WeightsAndBiases)
		wantErr string
	}{
		{"notifications omitted", func(*appsv2.WeightsAndBiases) {}, ""},
		{"complete Slack", func(w *appsv2.WeightsAndBiases) {
			w.Spec.Wandb.Notifications = &appsv2.NotificationsSpec{}
			w.Spec.Wandb.Notifications.Slack = &appsv2.SlackSpec{
				ClientID: notificationSelector("slack", "client-id"), ClientSecret: notificationSelector("slack", "client-secret"),
			}
		}, ""},
		{"Slack missing secret", func(w *appsv2.WeightsAndBiases) {
			w.Spec.Wandb.Notifications = &appsv2.NotificationsSpec{}
			w.Spec.Wandb.Notifications.Slack = &appsv2.SlackSpec{ClientID: notificationSelector("slack", "client-id")}
		}, "a value or secret reference is required"},
		{"email sink", func(w *appsv2.WeightsAndBiases) {
			w.Spec.Wandb.Notifications = &appsv2.NotificationsSpec{}
			w.Spec.Wandb.Notifications.Email = &appsv2.EmailSpec{Sink: &sink}
		}, ""},
		{"authenticated SMTP", func(w *appsv2.WeightsAndBiases) {
			w.Spec.Wandb.Notifications = &appsv2.NotificationsSpec{}
			w.Spec.Wandb.Notifications.Email = &appsv2.EmailSpec{SMTP: validSMTP()}
		}, ""},
		{"sink and SMTP", func(w *appsv2.WeightsAndBiases) {
			w.Spec.Wandb.Notifications = &appsv2.NotificationsSpec{}
			w.Spec.Wandb.Notifications.Email = &appsv2.EmailSpec{Sink: &sink, SMTP: validSMTP()}
		}, "exactly one"},
		{"neither email mode", func(w *appsv2.WeightsAndBiases) {
			w.Spec.Wandb.Notifications = &appsv2.NotificationsSpec{}
			w.Spec.Wandb.Notifications.Email = &appsv2.EmailSpec{}
		}, "exactly one"},
		{"SMTP missing password", func(w *appsv2.WeightsAndBiases) {
			smtp := validSMTP()
			smtp.Password = appsv2.ValueOrSecret{}
			w.Spec.Wandb.Notifications = &appsv2.NotificationsSpec{}
			w.Spec.Wandb.Notifications.Email = &appsv2.EmailSpec{SMTP: smtp}
		}, "a value or secret reference is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wandb := &appsv2.WeightsAndBiases{}
			tc.mutate(wandb)
			errs := validateNotificationSpec(wandb)
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("expected no errors, got %v", errs)
				}
				return
			}
			if len(errs) == 0 || !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, errs)
			}
		})
	}
}
