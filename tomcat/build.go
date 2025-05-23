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
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sbom"
	"github.com/paketo-buildpacks/libpak/sherpa"

	"github.com/heroku/color"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/apache-tomcat/v8/internal/util"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

type Build struct {
	Logger      bard.Logger
	SBOMScanner sbom.SBOMScanner
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	result := libcnb.NewBuildResult()

	pr := libpak.PlanEntryResolver{Plan: context.Plan}
	if _, found, err := pr.Resolve(PlanEntryJavaApplicationServer); err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to resolve plan entry\n%w", err)
	} else if !found {
		return result, nil
	}

	warFilesExist, _ := util.ContainsWarFiles(context.Application.Path)
	if warFilesExist {
		b.Logger.Infof("%s contains war files.", context.Application.Path)
	} else {
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
	}

	b.Logger.Title(context.Buildpack)

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
		v, versionExists := cr.Resolve("BP_TOMCAT_EXT_CONF_VERSION")
		s, shaExists := cr.Resolve("BP_TOMCAT_EXT_CONF_SHA256")

		if !versionExists && !shaExists {
			v = time.Now().Format(time.RFC3339)
			b.Logger.Infof(color.YellowString("WARNING: No BP_TOMCAT_EXT_CONF_VERSION or BP_TOMCAT_EXT_CONF_SHA256 provided, so no layer caching will occur."))
		}

		externalConfigurationDependency = &libpak.BuildpackDependency{
			ID:      "tomcat-external-configuration",
			Name:    "Tomcat External Configuration",
			Version: v,
			URI:     uri,
			SHA256:  s,
			Stacks:  []string{context.StackID},
			CPEs:    nil,
			PURL:    "",
		}
	}

	base, bomEntries := NewBase(context.Application.Path, context.Buildpack.Path, cr, b.ContextPath(cr), accessLoggingDependency, externalConfigurationDependency, lifecycleDependency, loggingDependency, dc, warFilesExist)

	base.Logger = b.Logger
	result.Layers = append(result.Layers, base)
	if bomEntries != nil {
		result.BOM.Entries = append(result.BOM.Entries, bomEntries...)
	}

	command := "sh"
	arguments := []string{filepath.Join(context.Layers.Path, "tomcat", "bin", "catalina.sh"), "run"}

	if libpak.IsTinyStack(context.StackID) {
		command, arguments = b.tinyStartCommand(
			filepath.Join(context.Layers.Path, "tomcat"),
			filepath.Join(context.Layers.Path, "catalina-base"),
			loggingDependency)
	}

	result.Processes = append(result.Processes,
		libcnb.Process{Type: "task", Command: command, Arguments: arguments, Direct: true},
		libcnb.Process{Type: "tomcat", Command: command, Arguments: arguments, Direct: true},
		libcnb.Process{Type: "web", Command: command, Arguments: arguments, Direct: true, Default: true},
	)

	if b.SBOMScanner == nil {
		b.SBOMScanner = sbom.NewSyftCLISBOMScanner(context.Layers, effect.CommandExecutor{}, b.Logger)
	}
	if err := b.SBOMScanner.ScanLaunch(context.Application.Path, libcnb.SyftJSON, libcnb.CycloneDXJSON); err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create Launch SBoM \n%w", err)
	}

	return result, nil
}

func (b Build) ContextPath(configurationResolver libpak.ConfigurationResolver) string {
	cp := "ROOT"
	if s, ok := configurationResolver.Resolve("BP_TOMCAT_CONTEXT_PATH"); ok {
		cp = s
	}
	cp = strings.TrimPrefix(cp, "/")
	cp = strings.TrimSuffix(cp, "/")
	cp = strings.ReplaceAll(cp, "/", "#")

	return cp
}

func (b Build) tinyStartCommand(homePath, basePath string, loggingDep libpak.BuildpackDependency) (string, []string) {
	command := "java"

	arguments := []string{
		fmt.Sprintf("-Djava.util.logging.config.file=%s/conf/logging.properties", basePath),
		"-Djava.util.logging.manager=org.apache.juli.ClassLoaderLogManager",
	}

	arguments = append(arguments, sherpa.GetEnvWithDefault("JSSE_OPTS", "-Djdk.tls.ephemeralDHKeySize=2048"))

	classpath := []string{
		fmt.Sprintf("%s/bin/%s", basePath, path.Base(loggingDep.URI)),
		fmt.Sprintf("%s/bin/bootstrap.jar", homePath),
		fmt.Sprintf("%s/bin/tomcat-juli.jar", homePath),
	}
	arguments = append(arguments, "-classpath", strings.Join(classpath, ":"))

	arguments = append(arguments,
		fmt.Sprintf("-Dcatalina.home=%s", homePath),
		fmt.Sprintf("-Dcatalina.base=%s", basePath),
		"org.apache.catalina.startup.Bootstrap", "start",
	)

	b.Logger.Header(color.YellowString("WARNING: Tomcat will run on the Tiny stack which has no shell. Due to this, some configuration options such as `bin/setenv.sh` and setting `CATALINA_*` environment variables, will not be available"))

	return command, arguments
}
