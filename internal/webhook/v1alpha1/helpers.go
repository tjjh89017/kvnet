/*
Copyright 2026.

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

package v1alpha1

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// validateMasterIsBridge checks that a master field references an existing BridgeTemplate's bridgeName.
func validateMasterIsBridge(ctx context.Context, c client.Client, master string) error {
	if master == "" {
		return nil
	}

	var list kvnetv1alpha1.BridgeTemplateList
	if err := c.List(ctx, &list); err != nil {
		return fmt.Errorf("failed to list BridgeTemplates: %w", err)
	}

	for i := range list.Items {
		if list.Items[i].Spec.BridgeName == master {
			return nil
		}
	}

	return fmt.Errorf("master %q does not match any BridgeTemplate's bridgeName", master)
}
