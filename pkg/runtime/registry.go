/*
Copyright 2022 cuisongliu@qq.com.

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

package runtime

import (
	"fmt"
	"path"

	"github.com/fanux/sealos/pkg/utils/contants"
	"github.com/fanux/sealos/pkg/utils/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fanux/sealos/pkg/passwd"
	"github.com/fanux/sealos/pkg/utils/file"

	"github.com/fanux/sealos/pkg/utils/logger"
)

const DefaultCPFmt = "mkdir -p %s && cp -rf  %s/* %s/"

func GetRegistry(rootfs, defaultRegistry string) *RegistryConfig {
	const registryCustomConfig = "registry.yml"
	var DefaultConfig = &RegistryConfig{
		IP:     defaultRegistry,
		Domain: contants.DefaultRegistryDomain,
		Port:   "5000",
	}
	etcPath := path.Join(rootfs, contants.EtcDirName, registryCustomConfig)
	registryConfig, err := yaml.Unmarshal(etcPath)
	if err != nil {
		logger.Debug("use default registry config")
		return DefaultConfig
	}
	domain, _, _ := unstructured.NestedString(registryConfig, "domain")
	port, _, _ := unstructured.NestedString(registryConfig, "port")
	username, _, _ := unstructured.NestedString(registryConfig, "username")
	password, _, _ := unstructured.NestedString(registryConfig, "password")
	data, _, _ := unstructured.NestedString(registryConfig, "data")
	ip, _, _ := unstructured.NestedString(registryConfig, "ip")

	if ip == "" {
		ip = defaultRegistry
	}
	if domain == "" {
		domain = DefaultConfig.Domain
	}
	if port == "" {
		domain = DefaultConfig.Port
	}
	rConfig := RegistryConfig{
		IP:       ip,
		Domain:   domain,
		Port:     port,
		Username: username,
		Password: password,
		Data:     data,
	}
	logger.Debug("show registry info, IP: %s, Domain: %s", rConfig.IP, rConfig.Domain)
	return &rConfig
}

func (k *KubeadmRuntime) htpasswd() error {
	htpasswdPath := path.Join(k.data.EtcPath(), "registry_htpasswd")
	registry := k.getRegistry()
	if registry.Username == "" && registry.Password == "" {
		return nil
	}
	data := passwd.Htpasswd(registry.Username, registry.Password)
	return file.WriteFile(htpasswdPath, []byte(data))
}

func (k *KubeadmRuntime) ApplyRegistry() error {
	logger.Info("start to apply registry")
	registry := k.getRegistry()
	err := k.sshCmdAsync(registry.IP, fmt.Sprintf(DefaultCPFmt, registry.Data, k.data.RootFSRegistryPath(), registry.Data))
	if err != nil {
		return fmt.Errorf("copy registry data failed %v", err)
	}
	ip := k.getMaster0IP()
	err = k.htpasswd()
	if err != nil {
		return fmt.Errorf("generator registry htpasswd failed %v", err)
	}
	err = k.execInitRegistry(ip)
	if err != nil {
		return fmt.Errorf("exec registry.sh failed %v", err)
	}

	return nil
}

func (k *KubeadmRuntime) DeleteRegistry() error {
	logger.Info("delete registry in master0...")
	ip := k.getMaster0IP()
	if err := k.execCleanRegistry(ip); err != nil {
		return fmt.Errorf("exec clean-registry.sh failed %v", err)
	}
	return nil
}

func (k *KubeadmRuntime) registryAuth(ip string) error {
	logger.Info("registry auth in node %s", ip)
	registry := k.getRegistry()
	err := k.execHostsAppend(ip, registry.IP, registry.Domain)
	if err != nil {
		return fmt.Errorf("add registry hosts failed %v", err)
	}

	err = k.execAuth(ip)
	if err != nil {
		return fmt.Errorf("exec auth.sh failed %v", err)
	}
	return nil
}