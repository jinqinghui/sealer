// Copyright © 2021 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package env

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	v2 "github.com/alibaba/sealer/types/api/v2"
	"github.com/alibaba/sealer/utils"
)

const templateSufix = ".tmpl"

type Interface interface {
	// WrapperShell :If host already set env like DATADISK=/data
	// This function add env to the shell, like:
	// Input shell: cat /etc/hosts
	// Output shell: DATADISK=/data cat /etc/hosts
	// So that you can get env values in you shell script
	WrapperShell(host, shell string) string
	// RenderAll :render env to all the files in dir
	RenderAll(host, dir string) error
}

type processor struct {
	*v2.Cluster
}

func NewEnvProcessor(cluster *v2.Cluster) Interface {
	return &processor{cluster}
}

func (p *processor) WrapperShell(host, shell string) string {
	var env string
	for k, v := range p.getHostEnv(host) {
		env = fmt.Sprintf("%s%s=%s ", env, k, v)
	}

	return fmt.Sprintf("%s%s", env, shell)
}

func (p *processor) RenderAll(host, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, errIn error) error {
		if errIn != nil {
			return errIn
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), templateSufix) {
			return nil
		}
		writer, err := os.OpenFile(strings.TrimSuffix(path, templateSufix), os.O_CREATE|os.O_RDWR, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to open file [%s] when render env: %v", path, err)
		}
		defer func() {
			_ = writer.Close()
		}()
		t, err := template.ParseFiles(path)
		if err != nil {
			return fmt.Errorf("failed to create template: %s %v", path, err)
		}
		if err := t.Execute(writer, p.getHostEnv(host)); err != nil {
			return fmt.Errorf("failed to render env template: %s %v", path, err)
		}
		return nil
	})
}

/*
func sameKey(keysrc string, list []string) bool {
	s := strings.SplitN(keysrc, "=", 2)
	if len(s) != 2 {
		return false
	}
	for _, l := range list {
		if strings.HasPrefix(l, s[0]) {
			return true
		}
	}
	return false
}
*/

func mergeList(dst, src []string) []string {
	for _, s := range src {
		if utils.InList(s, dst) {
			continue
		}
		dst = append(dst, s)
	}
	return dst
}

// Merge the host ENV and global env, the host env will overwrite cluster.Spec.Env
func (p *processor) getHostEnv(hostIP string) (env map[string]interface{}) {
	var hostEnv []string

	for _, host := range p.Spec.Hosts {
		for _, ip := range host.IPS {
			if ip == hostIP {
				hostEnv = host.Env
			}
		}
	}

	hostEnv = mergeList(hostEnv, p.Spec.Env)

	return convertEnv(hostEnv)
}

// Covert Env []string to map[string]interface{}, example [IP=127.0.0.1,IP=192.160.0.2,Key=value] will convert to {IP:[127.0.0.1,192.168.0.2],key:value}
func convertEnv(envList []string) (env map[string]interface{}) {
	temp := make(map[string][]string)
	env = make(map[string]interface{})

	for _, e := range envList {
		var kv []string
		if kv = strings.SplitN(e, "=", 2); len(kv) != 2 {
			continue
		}

		temp[kv[0]] = append(temp[kv[0]], kv[1])
	}

	for k, v := range temp {
		if len(v) > 1 {
			env[k] = v
			continue
		}
		if len(v) == 1 {
			env[k] = v[0]
		}
	}

	return
}