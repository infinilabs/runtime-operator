// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Runtime Operator is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package util

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/infinilabs/runtime-operator/pkg/apis/common"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

var (
	IsV1Supported bool
)

// GetInt32ValueOrDefault returns the value of an int32 pointer or a default value.
func GetInt32ValueOrDefault(ptr *int32, defaultValue int32) int32 {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// GetBoolValueOrDefault returns the value of a bool pointer or a default value.
func GetBoolValueOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// GetStringValueOrDefault returns the value of a string pointer or a default value.
func GetStringValueOrDefault(ptr *string, defaultValue string) string {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// --- Configuration Unmarshalling Helper ---
// Unmarshals raw ApplicationComponent.Properties based on the component type string
// into the correct specific Go configuration struct pointer.
// Returns the unmarshalled struct pointer (as interface{}) or nil if no properties are provided.
// Returns error if unmarshalling fails for the given type or type is unknown.
func UnmarshalAppSpecificConfig(appCompType string, rawProperties runtime.RawExtension) (interface{}, error) {
	if len(rawProperties.Raw) == 0 {
		// Return nil if no properties provided. Builders should handle defaults for nil config.
		return nil, nil
	}

	specificConfig := &common.RuntimeConfig{} // Target Go struct pointer

	// --- Perform Unmarshalling ---
	// Use json.Unmarshal as RawExtension contains JSON/YAML bytes.
	if err := json.Unmarshal(rawProperties.Raw, specificConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties for component type '%s' into expected type %T: %w", appCompType, specificConfig, err)
	}

	// Optional: Add post-unmarshalling validation specific to the type here if needed.

	return specificConfig, nil // Return the pointer to the unmarshalled specific struct
}

// SetK8sVersionGreaterOrEqual 检测集群版本是否 ≥ 目标版本（如 "1.21"）
func SetK8sVersionGreaterOrEqual(config *rest.Config, targetMajor, targetMinor int) {
	// 1. 创建 DiscoveryClient
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		panic(fmt.Errorf("failed to create discovery client: %v", err))
	}

	// 2. 获取服务器版本信息
	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		panic(fmt.Errorf("failed to get server version: %v", err))
	}

	// 3. 解析集群版本号（格式示例：v1.18.20）
	clusterMajor, clusterMinor, err := parseKubeVersion(serverVersion)
	if err != nil {
		panic(fmt.Errorf("failed to parse server version: %v", err))
	}

	// 4. 比较版本号
	IsV1Supported = clusterMajor > targetMajor ||
		(clusterMajor == targetMajor && clusterMinor >= targetMinor)
}

// 解析 Kubernetes 版本号（兼容带后缀的版本如 "v1.18+"）
func parseKubeVersion(v *version.Info) (major, minor int, err error) {
	// 去除前缀 "v" 和后缀非数字字符（如 "+"）
	cleanVer := strings.TrimPrefix(v.GitVersion, "v")
	cleanVer = strings.Split(cleanVer, "-")[0] // 忽略 "-gke.1" 等后缀
	parts := strings.Split(cleanVer, ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid version format: %s", v.GitVersion)
	}

	// 转换主版本和次版本为整数
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse major version: %v", err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse minor version: %v", err)
	}
	return major, minor, nil
}
