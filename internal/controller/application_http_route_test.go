package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	wandbv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var _ = Describe("Application HTTPRoute Helpers", func() {
	It("builds HTTPRoute rules from exact-match paths and backend ports", func() {
		servicePort := gatewayv1.PortNumber(8080)
		app := &wandbv2.Application{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb-api"},
			Spec: wandbv2.ApplicationSpec{
				HTTPRouteTemplate: &wandbv2.HTTPRouteTemplateSpec{
					Paths:       []string{"/api", "/graphql"},
					PathType:    "Exact",
					ServicePort: &servicePort,
				},
			},
		}

		rules := buildHTTPRouteRules(app)

		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Matches).To(HaveLen(2))
		Expect(*rules[0].Matches[0].Path.Type).To(Equal(gatewayv1.PathMatchExact))
		Expect(*rules[0].Matches[0].Path.Value).To(Equal("/api"))
		Expect(*rules[0].Matches[1].Path.Type).To(Equal(gatewayv1.PathMatchExact))
		Expect(*rules[0].Matches[1].Path.Value).To(Equal("/graphql"))
		Expect(rules[0].BackendRefs).To(HaveLen(1))
		Expect(rules[0].BackendRefs[0].Name).To(Equal(gatewayv1.ObjectName("wandb-api")))
		Expect(rules[0].BackendRefs[0].Port).NotTo(BeNil())
		Expect(*rules[0].BackendRefs[0].Port).To(Equal(gatewayv1.PortNumber(8080)))
	})

	It("marks an HTTPRoute accepted when any parent reports Accepted=True", func() {
		route := &gatewayv1.HTTPRoute{
			Status: gatewayv1.HTTPRouteStatus{
				RouteStatus: gatewayv1.RouteStatus{
					Parents: []gatewayv1.RouteParentStatus{
						{
							Conditions: []metav1.Condition{
								{
									Type:   string(gatewayv1.RouteConditionAccepted),
									Status: metav1.ConditionFalse,
								},
							},
						},
						{
							Conditions: []metav1.Condition{
								{
									Type:   string(gatewayv1.RouteConditionAccepted),
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
				},
			},
		}

		summary := summarizeHTTPRouteStatus(route)

		Expect(summary).NotTo(BeNil())
		Expect(summary.Accepted).To(BeTrue())
	})
})
