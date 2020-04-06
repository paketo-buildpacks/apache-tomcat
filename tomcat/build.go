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
	b.Logger.Body(bard.FormatUserConfig("BP_TOMCAT_CONTEXT_PATH", "the application context path", "ROOT"))
	b.Logger.Body(bard.FormatUserConfig("BP_TOMCAT_EXT_CONF_SHA256", "the SHA256 hash of the external Tomcat configuration archive", "<none>"))
	b.Logger.Body(bard.FormatUserConfig("BP_TOMCAT_EXT_CONF_STRIP", "the number of directory components to strip from the external Tomcat configuration archive", "0"))
	b.Logger.Body(bard.FormatUserConfig("BP_TOMCAT_EXT_CONF_URI", "the download location of the external Tomcat configuration", "<none>"))
	b.Logger.Body(bard.FormatUserConfig("BP_TOMCAT_EXT_CONF_VERSION", "the version of the external Tomcat configuration", "<none>"))
	b.Logger.Body(bard.FormatUserConfig("BP_TOMCAT_VERSION", "the Tomcat version", "9.*"))
	b.Logger.Body(bard.FormatUserConfig("BPL_TOMCAT_ACCESS_LOGGING", "the Tomcat access logging state", "disabled"))

	command := "catalina.sh run"
	result := libcnb.BuildResult{
		Processes: []libcnb.Process{
			{Type: "task", Command: command},
			{Type: "tomcat", Command: command},
			{Type: "web", Command: command},
		},
	}

	md, err := libpak.NewBuildpackMetadata(context.Buildpack.Metadata)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to unmarshal buildpack metadata\n%w", err)
	}

	dr, err := libpak.NewDependencyResolver(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency resolver\n%w", err)
	}

	dc := libpak.NewDependencyCache(context.Buildpack)
	dc.Logger = b.Logger

	v := md.DefaultVersions["tomcat"]
	if s, ok := os.LookupEnv("BP_TOMCAT_VERSION"); ok {
		v = s
	}
	tomcatDep, err := dr.Resolve("tomcat", v)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
	}

	home := NewHome(tomcatDep, dc, &result.Plan)
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
	if uri, ok := os.LookupEnv("BP_TOMCAT_EXT_CONF_URI"); ok {
		externalConfigurationDependency = &libpak.BuildpackDependency{
			ID:      "tomcat-external-configuration",
			Name:    "Tomcat External Configuration",
			Version: os.Getenv("BP_TOMCAT_EXT_CONF_VERSION"),
			URI:     uri,
			SHA256:  os.Getenv("BP_TOMCAT_EXT_CONF_SHA256"),
			Stacks:  []string{context.StackID},
		}
	}

	base := NewBase(context.Application.Path, context.Buildpack.Path, b.ContextPath(), accessLoggingDependency,
		externalConfigurationDependency, lifecycleDependency, loggingDependency, dc, &result.Plan)

	base.Logger = b.Logger
	result.Layers = append(result.Layers, base)

	return result, nil
}

func (Build) ContextPath() string {
	cp := "ROOT"
	if s, ok := os.LookupEnv("BP_TOMCAT_CONTEXT_PATH"); ok {
		cp = s
	}
	cp = strings.TrimPrefix(cp, "/")
	cp = strings.TrimSuffix(cp, "/")
	cp = strings.ReplaceAll(cp, "/", "#")

	return cp
}
