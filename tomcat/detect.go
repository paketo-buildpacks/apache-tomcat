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
	"github.com/paketo-buildpacks/apache-tomcat/v7/internal/util"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

const (
	PlanEntryJVMApplication        = "jvm-application"
	PlanEntryJVMApplicationPackage = "jvm-application-package"
	PlanEntryJRE                   = "jre"
	PlanEntrySyft                  = "syft"
	PlanEntryJavaApplicationServer = "java-app-server"
	JavaAppServerTomcat            = "tomcat"
)

type Detect struct {
	Logger bard.Logger
}

func (d Detect) Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &d.Logger)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	appServer, _ := cr.Resolve("BP_JAVA_APP_SERVER")
	if appServer != "" && appServer != JavaAppServerTomcat {
		d.Logger.Infof("SKIPPED: buildpack does not match requested app server of [%s], buildpack supports [%s]", appServer, JavaAppServerTomcat)
		return libcnb.DetectResult{Pass: false}, nil
	}

	warFilesExist, _ := util.ContainsWarFiles(context.Application.Path)
	if !warFilesExist {
		m, err := libjvm.NewManifest(context.Application.Path)
		if err != nil {
			return libcnb.DetectResult{}, fmt.Errorf("unable to read manifest\n%w", err)
		}

		if _, ok := m.Get("Main-Class"); ok {
			d.Logger.Info("SKIPPED: Manifest attribute 'Main-Class' was found")
			return libcnb.DetectResult{Pass: false}, nil
		}
	}

	result := libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryJVMApplication},
					{Name: PlanEntryJavaApplicationServer},
				},
				Requires: []libcnb.BuildPlanRequire{
					{Name: PlanEntrySyft},
					{Name: PlanEntryJRE, Metadata: map[string]interface{}{"launch": true}},
					{Name: PlanEntryJVMApplicationPackage},
					{Name: PlanEntryJVMApplication},
					{Name: PlanEntryJavaApplicationServer},
				},
			},
		},
	}

	file := filepath.Join(context.Application.Path, "WEB-INF")
	if _, err := os.Stat(file); err != nil && !os.IsNotExist(err) {
		return libcnb.DetectResult{}, fmt.Errorf("unable to stat file %s\n%w", file, err)
	} else if os.IsNotExist(err) {
		d.Logger.Info("PASSED: a WEB-INF directory was not found, this is normal when building from source")
		return result, nil
	}

	result.Plans[0].Provides = append(result.Plans[0].Provides, libcnb.BuildPlanProvide{Name: PlanEntryJVMApplicationPackage})
	return result, nil
}
