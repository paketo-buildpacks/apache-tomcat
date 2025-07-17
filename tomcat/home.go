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
	"slices"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/crush"
)

type Home struct {
	LayerContributor libpak.DependencyLayerContributor
	Logger           bard.Logger
}

func NewHome(dependency libpak.BuildpackDependency, cache libpak.DependencyCache) (Home, libcnb.BOMEntry) {
	contrib, entry := libpak.NewDependencyLayer(dependency, cache, libcnb.LayerTypes{
		Launch: true,
	})
	return Home{LayerContributor: contrib}, entry
}

func (h Home) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	h.LayerContributor.Logger = h.Logger

	return h.LayerContributor.Contribute(layer, func(artifact *os.File) (libcnb.Layer, error) {
		h.Logger.Bodyf("Expanding to %s", layer.Path)
		if err := crush.Extract(artifact, layer.Path, 1); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to expand Tomcat\n%w", err)
		}

		err := h.relaxPermissions(layer)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to relax permissions\n%w", err)
		}

		layer.LaunchEnvironment.Default("CATALINA_HOME", layer.Path)

		return layer, nil
	})
}

type relaxPath struct {
	path       string
	extensions []string
}

func (h Home) relaxPermissions(layer libcnb.Layer) error {
	relaxPermissions, ok := os.LookupEnv("BP_TOMCAT_RELAX_PERMISSIONS")
	if ok && relaxPermissions == "true" {

		var relaxPaths = [...]relaxPath{
			{filepath.Join(layer.Path, "bin"), []string{".sh", ".jar"}},
			{filepath.Join(layer.Path, "lib"), []string{".jar"}},
		}

		for _, path := range relaxPaths {
			err := h.relaxFiles(path.path, path.extensions...)
			if err != nil {
				return fmt.Errorf("unable to relax relaxPermissions on %q files\n%w", path.path, err)
			}
		}

	}
	return nil
}

func (h Home) relaxFiles(workpath string, ext ...string) error {
	return filepath.Walk(workpath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}

		if !info.IsDir() && slices.Contains(ext, filepath.Ext(path)) {
			h.Logger.Bodyf("Relaxing permissions on file: %s", path)
			err = os.Chmod(path, 0755)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (h Home) Name() string {
	return h.LayerContributor.LayerName()
}
