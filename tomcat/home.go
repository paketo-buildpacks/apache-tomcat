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
		if err := crush.ExtractTarGz(artifact, layer.Path, 1); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to expand Tomcat\n%w", err)
		}
		if err := os.Chmod(filepath.Join(layer.Path), 0775); err != nil{
			return  libcnb.Layer{}, fmt.Errorf("unable to set catalina home dir permissions\n%w", err)
		}

		layer.LaunchEnvironment.Default("CATALINA_HOME", layer.Path)

		return layer, nil
	})
}

func (h Home) Name() string {
	return h.LayerContributor.LayerName()
}
