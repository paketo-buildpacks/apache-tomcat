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

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

type Build struct {
	Logger bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	file := filepath.Join(context.Application.Path, "WEB-INF")
	if _, err := os.Stat(file); err != nil && !os.IsNotExist(err) {
		return libcnb.BuildResult{}, fmt.Errorf("unable to stat file %s\n%w", file, err)
	} else if os.IsNotExist(err) {
		return libcnb.BuildResult{}, nil
	}

	b.Logger.Title(context.Buildpack)
	result := libcnb.NewBuildResult()

	command := "catalina.sh run"
	result.Processes = append(result.Processes,
		libcnb.Process{Type: "task", Command: command},
		libcnb.Process{Type: "tomcat", Command: command},
		libcnb.Process{Type: "web", Command: command},
	)

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

	home := NewHome(tomcatDep, dc, result.Plan)
	home.Logger = b.Logger
	result.Layers = append(result.Layers, home)

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

	base := NewBase(context.Application.Path, context.Buildpack.Path, cr, b.ContextPath(cr), accessLoggingDependency,
		externalConfigurationDependency, lifecycleDependency, loggingDependency, dc, result.Plan)

	base.Logger = b.Logger
	result.Layers = append(result.Layers, base)

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
