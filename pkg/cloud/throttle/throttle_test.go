/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package throttle

import (
	"context"
	"testing"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api-provider-aws/v2/pkg/internal/rate"
)

func TestServiceLimiterReviewResponse(t *testing.T) {
	ctx := operationContext(t, "DescribeCluster")

	tests := []struct {
		name      string
		errorCode string
		wantReset bool
	}{
		{
			name:      "EKS throttling error resets tokens",
			errorCode: "TooManyRequestsException",
			wantReset: true,
		},
		{
			name:      "EC2 throttling error resets tokens",
			errorCode: "Throttling",
			wantReset: true,
		},
		{
			name:      "unrelated error does not reset tokens",
			errorCode: "AccessDenied",
			wantReset: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			operationLimiter := &OperationLimiter{
				Operation:  "DescribeCluster",
				RefillRate: rate.Limit(1),
				Burst:      1,
			}
			limiter := operationLimiter.getLimiter()
			limiter.SetBurst(1)

			ServiceLimiter{operationLimiter}.ReviewResponse(ctx, tt.errorCode)

			reservation := limiter.Reserve()
			if tt.wantReset {
				g.Expect(reservation.Delay()).To(BeNumerically(">", 0))
			} else {
				g.Expect(reservation.Delay()).To(BeZero())
			}
		})
	}
}

func operationContext(t *testing.T, operationName string) context.Context {
	t.Helper()

	var operationContext context.Context
	metadata := awsmiddleware.RegisterServiceMetadata{OperationName: operationName}
	_, _, err := metadata.HandleInitialize(context.Background(), middleware.InitializeInput{}, middleware.InitializeHandlerFunc(func(ctx context.Context, _ middleware.InitializeInput) (middleware.InitializeOutput, middleware.Metadata, error) {
		operationContext = ctx
		return middleware.InitializeOutput{}, middleware.Metadata{}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	return operationContext
}
