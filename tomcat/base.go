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
	"strconv"

	"github.com/paketo-buildpacks/libpak/sbom"

	"github.com/buildpacks/libcnb"
	"github.com/heroku/color"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/crush"
	"github.com/paketo-buildpacks/libpak/sherpa"
)

type Base struct {
	AccessLoggingDependency         libpak.BuildpackDependency
	ApplicationPath                 string
	BuildpackPath                   string
	ConfigurationResolver           libpak.ConfigurationResolver
	ContextPath                     string
	DependencyCache                 libpak.DependencyCache
	ExternalConfigurationDependency *libpak.BuildpackDependency
	LayerContributor                libpak.LayerContributor
	LifecycleDependency             libpak.BuildpackDependency
	LoggingDependency               libpak.BuildpackDependency
	Logger                          bard.Logger
}

func NewBase(
	applicationPath string,
	buildpackPath string,
	configurationResolver libpak.ConfigurationResolver,
	contextPath string,
	accessLoggingDependency libpak.BuildpackDependency,
	externalConfigurationDependency *libpak.BuildpackDependency,
	lifecycleDependency libpak.BuildpackDependency,
	loggingDependency libpak.BuildpackDependency,
	cache libpak.DependencyCache,
) (Base, []libcnb.BOMEntry) {

	dependencies := []libpak.BuildpackDependency{accessLoggingDependency, lifecycleDependency, loggingDependency}
	if externalConfigurationDependency != nil {
		dependencies = append(dependencies, *externalConfigurationDependency)
	}

	b := Base{
		AccessLoggingDependency:         accessLoggingDependency,
		ApplicationPath:                 applicationPath,
		BuildpackPath:                   buildpackPath,
		ConfigurationResolver:           configurationResolver,
		ContextPath:                     contextPath,
		DependencyCache:                 cache,
		ExternalConfigurationDependency: externalConfigurationDependency,
		LayerContributor: libpak.NewLayerContributor("Apache Tomcat Support", map[string]interface{}{
			"context-path": contextPath,
			"dependencies": dependencies,
		}, libcnb.LayerTypes{
			Launch: true,
		}),
		LifecycleDependency: lifecycleDependency,
		LoggingDependency:   loggingDependency,
	}

	var bomEntries []libcnb.BOMEntry

	var entry libcnb.BOMEntry
	entry = accessLoggingDependency.AsBOMEntry()
	entry.Metadata["layer"] = b.Name()
	entry.Launch = true
	bomEntries = append(bomEntries, entry)

	entry = lifecycleDependency.AsBOMEntry()
	entry.Metadata["layer"] = b.Name()
	entry.Launch = true
	bomEntries = append(bomEntries, entry)

	entry = loggingDependency.AsBOMEntry()
	entry.Metadata["layer"] = b.Name()
	entry.Launch = true
	bomEntries = append(bomEntries, entry)

	if externalConfigurationDependency != nil {
		entry = externalConfigurationDependency.AsBOMEntry()
		entry.Metadata["layer"] = b.Name()
		entry.Launch = true
		bomEntries = append(bomEntries, entry)
	}

	return b, bomEntries
}

func (b Base) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	b.LayerContributor.Logger = b.Logger
	var syftArtifacts []sbom.SyftArtifact

	return b.LayerContributor.Contribute(layer, func() (libcnb.Layer, error) {

		if err := b.ContributeConfiguration(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute configuration\n%w", err)
		}

		if err := b.ContributeAccessLogging(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute access logging\n%w", err)
		}
		if syftArtifact, err := b.AccessLoggingDependency.AsSyftArtifact(); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to get Syft Artifact for dependency: %s, \n%w", b.AccessLoggingDependency.Name, err)
		} else {
			syftArtifacts = append(syftArtifacts, syftArtifact)
		}

		if err := b.ContributeLifecycle(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute lifecycle\n%w", err)
		}
		if syftArtifact, err := b.LifecycleDependency.AsSyftArtifact(); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to get Syft Artifact for dependency: %s, \n%w", b.LifecycleDependency.Name, err)
		} else {
			syftArtifacts = append(syftArtifacts, syftArtifact)
		}

		if err := b.ContributeLogging(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute logging\n%w", err)
		}
		if syftArtifact, err := b.LoggingDependency.AsSyftArtifact(); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to get Syft Artifact for dependency: %s, \n%w", b.LoggingDependency.Name, err)
		} else {
			syftArtifacts = append(syftArtifacts, syftArtifact)
		}

		if b.ExternalConfigurationDependency != nil {
			if err := b.ContributeExternalConfiguration(layer); err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to contribute external configuration\n%w", err)
			}
			if syftArtifact, err := b.ExternalConfigurationDependency.AsSyftArtifact(); err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to get Syft Artifact for dependency: %s, \n%w", b.ExternalConfigurationDependency.Name, err)
			} else {
				syftArtifacts = append(syftArtifacts, syftArtifact)
			}
		}

		file := filepath.Join(layer.Path, "temp")
		if err := os.MkdirAll(file, 0700); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to create directory %s\n%w", file, err)
		}

		file = filepath.Join(layer.Path, "webapps")
		if err := os.MkdirAll(file, 0755); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to create directory %s\n%w", file, err)
		}

		file = filepath.Join(layer.Path, "webapps", b.ContextPath)
		b.Logger.Headerf("Mounting application at %s", b.ContextPath)
		if err := os.Symlink(b.ApplicationPath, file); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to create symlink from %s to %s\n%w", b.ApplicationPath, file, err)
		}

		environmentPropertySourceDisabled := b.ConfigurationResolver.ResolveBool("BP_TOMCAT_ENV_PROPERTY_SOURCE_DISABLED")
		if !environmentPropertySourceDisabled {
			layer.LaunchEnvironment.Default("CATALINA_OPTS", "-Dorg.apache.tomcat.util.digester.PROPERTY_SOURCE=org.apache.tomcat.util.digester.EnvironmentPropertySource")
		}

		layer.LaunchEnvironment.Default("CATALINA_BASE", layer.Path)
		layer.LaunchEnvironment.Default("CATALINA_TMPDIR", "/tmp")

		if err := b.writeDependencySBOM(layer, syftArtifacts); err != nil {
			return libcnb.Layer{}, err
		}

		return layer, nil
	})
}

func (b Base) ContributeAccessLogging(layer libcnb.Layer) error {
	b.Logger.Header(color.BlueString("%s %s", b.AccessLoggingDependency.Name, b.AccessLoggingDependency.Version))

	artifact, err := b.DependencyCache.Artifact(b.AccessLoggingDependency)
	if err != nil {
		return fmt.Errorf("unable to get dependency %s\n%w", b.AccessLoggingDependency.ID, err)
	}
	defer artifact.Close()

	b.Logger.Bodyf("Copying to %s/lib", layer.Path)

	file := filepath.Join(layer.Path, "lib", filepath.Base(artifact.Name()))
	if err := sherpa.CopyFile(artifact, file); err != nil {
		return fmt.Errorf("unable to copy %s to %s\n%w", artifact.Name(), file, err)
	}

	return nil
}

func (b Base) ContributeConfiguration(layer libcnb.Layer) error {
	file := filepath.Join(layer.Path, "conf")
	if err := os.MkdirAll(file, 0755); err != nil {
		return fmt.Errorf("unable to create directory %s\n%w", file, err)
	}

	b.Logger.Bodyf("Copying context.xml to %s/conf", layer.Path)
	file = filepath.Join(b.BuildpackPath, "resources", "context.xml")
	in, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("unable to open %s\n%w", file, err)
	}
	defer in.Close()

	file = filepath.Join(layer.Path, "conf", "context.xml")
	if err := sherpa.CopyFile(in, file); err != nil {
		return fmt.Errorf("unable to copy %s to %s\n%w", in.Name(), file, err)
	}

	b.Logger.Bodyf("Copying logging.properties to %s/conf", layer.Path)
	file = filepath.Join(b.BuildpackPath, "resources", "logging.properties")
	in, err = os.Open(file)
	if err != nil {
		return fmt.Errorf("unable to open %s\n%w", file, err)
	}
	defer in.Close()

	file = filepath.Join(layer.Path, "conf", "logging.properties")
	if err := sherpa.CopyFile(in, file); err != nil {
		return fmt.Errorf("unable to copy %s to %s\n%w", in.Name(), file, err)
	}

	b.Logger.Bodyf("Copying server.xml to %s/conf", layer.Path)
	file = filepath.Join(b.BuildpackPath, "resources", "server.xml")
	in, err = os.Open(file)
	if err != nil {
		return fmt.Errorf("unable to open %s\n%w", file, err)
	}
	defer in.Close()

	file = filepath.Join(layer.Path, "conf", "server.xml")
	if err := sherpa.CopyFile(in, file); err != nil {
		return fmt.Errorf("unable to copy %s to %s\n%w", in.Name(), file, err)
	}

	b.Logger.Bodyf("Copying web.xml to %s/conf", layer.Path)
	file = filepath.Join(b.BuildpackPath, "resources", "web.xml")
	in, err = os.Open(file)
	if err != nil {
		return fmt.Errorf("unable to open %s\n%w", file, err)
	}
	defer in.Close()

	file = filepath.Join(layer.Path, "conf", "web.xml")
	if err := sherpa.CopyFile(in, file); err != nil {
		return fmt.Errorf("unable to copy %s to %s\n%w", in.Name(), file, err)
	}

	return nil
}

func (b Base) ContributeExternalConfiguration(layer libcnb.Layer) error {
	b.Logger.Header(color.BlueString("%s %s", b.ExternalConfigurationDependency.Name, b.ExternalConfigurationDependency.Version))

	artifact, err := b.DependencyCache.Artifact(*b.ExternalConfigurationDependency)
	if err != nil {
		return fmt.Errorf("unable to get dependency %s\n%w", b.ExternalConfigurationDependency.ID, err)
	}
	defer artifact.Close()

	b.Logger.Bodyf("Expanding to %s", layer.Path)

	c := 0
	if s, ok := b.ConfigurationResolver.Resolve("BP_TOMCAT_EXT_CONF_STRIP"); ok {
		if c, err = strconv.Atoi(s); err != nil {
			return fmt.Errorf("unable to parse %s to integer\n%w", s, err)
		}
	}

	if err := crush.ExtractTarGz(artifact, layer.Path, c); err != nil {
		return fmt.Errorf("unable to expand external configuration\n%w", err)
	}

	return nil
}

func (b Base) ContributeLifecycle(layer libcnb.Layer) error {
	b.Logger.Header(color.BlueString("%s %s", b.LifecycleDependency.Name, b.LifecycleDependency.Version))

	artifact, err := b.DependencyCache.Artifact(b.LifecycleDependency)
	if err != nil {
		return fmt.Errorf("unable to get dependency %s\n%w", b.LifecycleDependency.ID, err)
	}
	defer artifact.Close()

	b.Logger.Bodyf("Copying to %s/lib", layer.Path)

	file := filepath.Join(layer.Path, "lib", filepath.Base(artifact.Name()))
	if err := sherpa.CopyFile(artifact, file); err != nil {
		return fmt.Errorf("unable to copy %s to %s\n%w", artifact.Name(), file, err)
	}

	return nil
}

func (b Base) ContributeLogging(layer libcnb.Layer) error {
	b.Logger.Header(color.BlueString("%s %s", b.LoggingDependency.Name, b.LoggingDependency.Version))

	artifact, err := b.DependencyCache.Artifact(b.LoggingDependency)
	if err != nil {
		return fmt.Errorf("unable to get dependency %s\n%w", b.LoggingDependency.ID, err)
	}
	defer artifact.Close()

	b.Logger.Bodyf("Copying to %s/bin", layer.Path)

	file := filepath.Join(layer.Path, "bin", filepath.Base(artifact.Name()))
	if err := sherpa.CopyFile(artifact, file); err != nil {
		return fmt.Errorf("unable to copy %s to %s\n%w", artifact.Name(), file, err)
	}

	b.Logger.Bodyf("Writing %s/bin/setenv.sh", layer.Path)

	s := fmt.Sprintf(`CLASSPATH="%s"`, file)

	file = filepath.Join(layer.Path, "bin", "setenv.sh")
	if err = os.WriteFile(file, []byte(s), 0755); err != nil {
		return fmt.Errorf("unable to write file %s\n%w", file, err)
	}

	return nil
}

func (b Base) writeDependencySBOM(layer libcnb.Layer, syftArtifacts []sbom.SyftArtifact) error {

	sbomPath := layer.SBOMPath(libcnb.SyftJSON)
	dep := sbom.NewSyftDependency(layer.Path, syftArtifacts)
	b.Logger.Debugf("Writing Syft SBOM at %s: %+v", sbomPath, dep)
	if err := dep.WriteTo(sbomPath); err != nil {
		return fmt.Errorf("unable to write SBOM\n%w", err)
	}
	return nil
}

func (Base) Name() string {
	return "catalina-base"
}
