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
	"strings"

	"github.com/heroku/color"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

type Build struct {
	Logger bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	result := libcnb.NewBuildResult()
	m, err := libjvm.NewManifest(context.Application.Path)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to read manifest\n%w", err)
	}

	if _, ok := m.Get("Main-Class"); ok {
		for _, entry := range context.Plan.Entries {
			result.Unmet = append(result.Unmet, libcnb.UnmetPlanEntry{Name: entry.Name})
		}
		return result, nil
	}

	file := filepath.Join(context.Application.Path, "WEB-INF")
	if _, err := os.Stat(file); err != nil && !os.IsNotExist(err) {
		return libcnb.BuildResult{}, fmt.Errorf("unable to stat file %s\n%w", file, err)
	} else if os.IsNotExist(err) {
		for _, entry := range context.Plan.Entries {
			result.Unmet = append(result.Unmet, libcnb.UnmetPlanEntry{Name: entry.Name})
		}
		return result, nil
	}

	b.Logger.Title(context.Buildpack)

	if context.StackID == libpak.TinyStackID {

		homePath := filepath.Join(context.Layers.Path, "tomcat")
		basePath := filepath.Join(context.Layers.Path, "catalina-base")
		command := "java"

		arguments := []string{
			"-classpath",
			fmt.Sprintf("%[1]s/bin/bootstrap.jar:%[1]s/bin/tomcat-juli.jar", homePath, homePath),
			fmt.Sprintf("-Dcatalina.home=%s", homePath),
			fmt.Sprintf("-Dcatalina.base=%s", basePath),
			fmt.Sprintf("-Djava.io.tmpdir=%s", filepath.Join(basePath, "/temp")),
			"org.apache.catalina.startup.Bootstrap", "start"}
		result.Processes = append(result.Processes,
			libcnb.Process{Type: "task", Command: command, Arguments: arguments, Direct: true},
			libcnb.Process{Type: "tomcat", Command: command, Arguments: arguments, Direct: true},
			libcnb.Process{Type: "web", Command: command, Arguments: arguments, Default: true, Direct: true},
		)
		b.Logger.Header(color.YellowString("WARNING: Tomcat will run on the Tiny stack which has no shell, some configuration options, such as setting CATALINA_OPTS, will not be available"))
	} else {
		command := "catalina.sh"
		arguments := []string{"run"}
		result.Processes = append(result.Processes,
			libcnb.Process{Type: "task", Command: command, Arguments: arguments},
			libcnb.Process{Type: "tomcat", Command: command, Arguments: arguments},
			libcnb.Process{Type: "web", Command: command, Arguments: arguments, Default: true},
		)
	}

	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &b.Logger)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	dr, err := libpak.NewDependencyResolver(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency resolver\n%w", err)
	}

	dc, err := libpak.NewDependencyCache(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency cache\n%w", err)
	}
	dc.Logger = b.Logger

	v, _ := cr.Resolve("BP_TOMCAT_VERSION")
	tomcatDep, err := dr.Resolve("tomcat", v)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
	}

	if strings.HasPrefix(tomcatDep.Version, "7") {
		dc.Logger.Headerf("%s Apache Tomcat version 7 is End-of-Life, this is the last release that will contain version 7", color.YellowString("WARNING: "))
	}

	home, be := NewHome(tomcatDep, dc)
	home.Logger = b.Logger
	result.Layers = append(result.Layers, home)
	result.BOM.Entries = append(result.BOM.Entries, be)

	h, be := libpak.NewHelperLayer(context.Buildpack, "access-logging-support")
	h.Logger = b.Logger
	result.Layers = append(result.Layers, h)
	result.BOM.Entries = append(result.BOM.Entries, be)

	accessLoggingDependency, err := dr.Resolve("tomcat-access-logging-support", "")
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
	}

	lifecycleDependency, err := dr.Resolve("tomcat-lifecycle-support", "")
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
	}

	loggingDependency, err := dr.Resolve("tomcat-logging-support", "")
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
	}

	var externalConfigurationDependency *libpak.BuildpackDependency
	if uri, ok := cr.Resolve("BP_TOMCAT_EXT_CONF_URI"); ok {
		v, _ := cr.Resolve("BP_TOMCAT_EXT_CONF_VERSION")
		s, _ := cr.Resolve("BP_TOMCAT_EXT_CONF_SHA256")

		externalConfigurationDependency = &libpak.BuildpackDependency{
			ID:      "tomcat-external-configuration",
			Name:    "Tomcat External Configuration",
			Version: v,
			URI:     uri,
			SHA256:  s,
			Stacks:  []string{context.StackID},
		}
	}

	base, bomEntries := NewBase(context.Application.Path, context.Buildpack.Path, cr, b.ContextPath(cr), accessLoggingDependency, externalConfigurationDependency, lifecycleDependency, loggingDependency, dc)

	base.Logger = b.Logger
	result.Layers = append(result.Layers, base)
	result.BOM.Entries = append(result.BOM.Entries, bomEntries...)

	return result, nil
}

func (Build) ContextPath(configurationResolver libpak.ConfigurationResolver) string {
	cp := "ROOT"
	if s, ok := configurationResolver.Resolve("BP_TOMCAT_CONTEXT_PATH"); ok {
		cp = s
	}
	cp = strings.TrimPrefix(cp, "/")
	cp = strings.TrimSuffix(cp, "/")
	cp = strings.ReplaceAll(cp, "/", "#")

	return cp
}
