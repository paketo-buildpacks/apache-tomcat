/*
 * Copyright 2018-2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tomcat

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb"
)

type Detect struct{}

func (d Detect) Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	result := libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
			{
				Requires: []libcnb.BuildPlanRequire{
					{Name: "jre", Metadata: map[string]interface{}{"launch": true}},
					{Name: "jvm-application"},
				},
			},
		},
	}

	file := filepath.Join(context.Application.Path, "WEB-INF")
	if _, err := os.Stat(file); err != nil && !os.IsNotExist(err) {
		return libcnb.DetectResult{}, fmt.Errorf("unable to stat file %s\n%w", file, err)
	} else if !os.IsNotExist(err) {
		result.Plans[0].Provides = append(result.Plans[0].Provides, libcnb.BuildPlanProvide{Name: "jvm-application"})
	}

	return result, nil
}
