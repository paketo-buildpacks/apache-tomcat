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

package tomcat_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/apache-tomcat/v8/tomcat"
)

func testBase(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx libcnb.BuildContext
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = os.MkdirTemp("", "base-application")
		Expect(err).NotTo(HaveOccurred())

		ctx.Buildpack.Path, err = os.MkdirTemp("", "base-buildpack")
		Expect(err).NotTo(HaveOccurred())

		ctx.Layers.Path, err = os.MkdirTemp("", "base-layers")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(filepath.Join(ctx.Layers.Path, "tomcat", "conf"), 0755)).To(Succeed())
		commonLoader := "common.loader=\"${catalina.base}/lib\",\"${catalina.base}/lib/*.jar\",\"${catalina.home}/lib\",\"${catalina.home}/lib/*.jar\""
		Expect(os.WriteFile(filepath.Join(ctx.Layers.Path, "tomcat", "conf", "catalina.properties"), []byte(commonLoader), 0644)).
			To(Succeed())

		Expect(os.MkdirAll(filepath.Join(ctx.Buildpack.Path, "resources"), 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Buildpack.Path, "resources", "context.xml"), []byte{}, 0644)).
			To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Buildpack.Path, "resources", "logging.properties"), []byte{}, 0644)).
			To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Buildpack.Path, "resources", "server.xml"), []byte{}, 0644)).
			To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Buildpack.Path, "resources", "web.xml"), []byte{}, 0644)).
			To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Buildpack.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("contributes catalina base", func() {
		accessLoggingDep := libpak.BuildpackDependency{
			ID:     "tomcat-access-logging-support",
			URI:    "https://localhost/stub-tomcat-access-logging-support.jar",
			SHA256: "d723bfe2ba67dfa92b24e3b6c7b2d0e6a963de7313350e306d470e44e330a5d2",
			PURL:   "pkg:generic/tomcat-access-logging-support@3.3.0",
			CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-access-logging-support:3.3.0:*:*:*:*:*:*:*"},
		}
		lifecycleDep := libpak.BuildpackDependency{
			ID:     "tomcat-lifecycle-support",
			URI:    "https://localhost/stub-tomcat-lifecycle-support.jar",
			SHA256: "723126712c0b22a7fe409664adf1fbb78cf3040e313a82c06696f5058e190534",
			PURL:   "pkg:generic/tomcat-lifecycle-support@3.3.0",
			CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-lifecycle-support:3.3.0:*:*:*:*:*:*:*"},
		}
		loggingDep := libpak.BuildpackDependency{
			ID:     "tomcat-logging-support",
			URI:    "https://localhost/stub-tomcat-logging-support.jar",
			SHA256: "e0a7e163cc9f1ffd41c8de3942c7c6b505090b7484c2ba9be846334e31c44a2c",
			PURL:   "pkg:generic/tomcat-logging-support@3.3.0",
			CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-logging-support:3.3.0:*:*:*:*:*:*:*"},
		}

		dc := libpak.DependencyCache{CachePath: "testdata"}

		contributor, entries := tomcat.NewBase(
			ctx.Application.Path,
			ctx.Buildpack.Path,
			libpak.ConfigurationResolver{},
			"test-context-path",
			accessLoggingDep,
			nil,
			lifecycleDep,
			loggingDep,
			dc,
			false,
		)

		Expect(entries).To(HaveLen(3))
		Expect(entries[0].Name).To(Equal("tomcat-access-logging-support"))
		Expect(entries[0].Build).To(BeFalse())
		Expect(entries[0].Launch).To(BeTrue())
		Expect(entries[1].Name).To(Equal("tomcat-lifecycle-support"))
		Expect(entries[1].Build).To(BeFalse())
		Expect(entries[1].Launch).To(BeTrue())
		Expect(entries[2].Name).To(Equal("tomcat-logging-support"))
		Expect(entries[2].Build).To(BeFalse())
		Expect(entries[2].Launch).To(BeTrue())

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = contributor.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Launch).To(BeTrue())
		Expect(filepath.Join(layer.Path, "conf", "context.xml")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "conf", "logging.properties")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "conf", "server.xml")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "conf", "web.xml")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "conf", "catalina.properties")).To(BeARegularFile())
		Expect(os.ReadFile(filepath.Join(layer.Path, "conf", "catalina.properties"))).To(ContainSubstring("common.loader=${BPI_TOMCAT_ADDITIONAL_COMMON_JARS}"))
		Expect(filepath.Join(layer.Path, "lib", "stub-tomcat-access-logging-support.jar")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "lib", "stub-tomcat-lifecycle-support.jar")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "bin", "stub-tomcat-logging-support.jar")).To(BeARegularFile())
		Expect(os.ReadFile(filepath.Join(layer.Path, "bin", "setenv.sh"))).To(Equal(
			[]byte(fmt.Sprintf(`CLASSPATH="%s"`, filepath.Join(layer.Path, "bin", "stub-tomcat-logging-support.jar")))))
		Expect(layer.LaunchEnvironment["CATALINA_BASE.default"]).To(Equal(layer.Path))
		Expect(filepath.Join(layer.Path, "temp")).To(BeADirectory())

		file := filepath.Join(layer.Path, "webapps", "test-context-path")
		fi, err := os.Lstat(file)
		Expect(err).NotTo(HaveOccurred())
		Expect(fi.Mode() & os.ModeSymlink).To(Equal(os.ModeSymlink))
		Expect(os.Readlink(file)).To(Equal(ctx.Application.Path))

		Expect(layer.LaunchEnvironment["CATALINA_BASE.default"]).To(Equal(layer.Path))
		Expect(layer.LaunchEnvironment["CATALINA_OPTS.default"]).To(Equal("-DBPI_TOMCAT_ADDITIONAL_COMMON_JARS=${BPI_TOMCAT_ADDITIONAL_COMMON_JARS} -Dorg.apache.tomcat.util.digester.PROPERTY_SOURCE=org.apache.tomcat.util.digester.EnvironmentPropertySource"))
	})

	it("contributes custom configuration", func() {
		externalConfigurationDep := libpak.BuildpackDependency{
			ID:     "tomcat-external-configuration",
			URI:    "https://localhost/stub-external-configuration.tar.gz",
			SHA256: "22e708cfd301430cbcf8d1c2289503d8288d50df519ff4db7cca0ff9fe83c324",
			PURL:   "pkg:generic/tomcat@1.1.1",
			CPEs:   []string{"cpe:2.3:a:apache:tomcat:1.1.1:*:*:*:*:*:*:*"},
		}
		accessLoggingDep := libpak.BuildpackDependency{
			ID:     "tomcat-access-logging-support",
			URI:    "https://localhost/stub-tomcat-access-logging-support.jar",
			SHA256: "d723bfe2ba67dfa92b24e3b6c7b2d0e6a963de7313350e306d470e44e330a5d2",
			PURL:   "pkg:generic/tomcat-access-logging-support@3.3.0",
			CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-access-logging-support:3.3.0:*:*:*:*:*:*:*"},
		}
		lifecycleDep := libpak.BuildpackDependency{
			ID:     "tomcat-lifecycle-support",
			URI:    "https://localhost/stub-tomcat-lifecycle-support.jar",
			SHA256: "723126712c0b22a7fe409664adf1fbb78cf3040e313a82c06696f5058e190534",
			PURL:   "pkg:generic/tomcat-lifecycle-support@3.3.0",
			CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-lifecycle-support:3.3.0:*:*:*:*:*:*:*"},
		}
		loggingDep := libpak.BuildpackDependency{
			ID:     "tomcat-logging-support",
			URI:    "https://localhost/stub-tomcat-logging-support.jar",
			SHA256: "e0a7e163cc9f1ffd41c8de3942c7c6b505090b7484c2ba9be846334e31c44a2c",
			PURL:   "pkg:generic/tomcat-logging-support@3.3.0",
			CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-logging-support:3.3.0:*:*:*:*:*:*:*"},
		}

		dc := libpak.DependencyCache{CachePath: "testdata"}

		contrib, entries := tomcat.NewBase(
			ctx.Application.Path,
			ctx.Buildpack.Path,
			libpak.ConfigurationResolver{},
			"test-context-path",
			accessLoggingDep,
			&externalConfigurationDep,
			lifecycleDep,
			loggingDep,
			dc,
			false,
		)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		Expect(entries).To(HaveLen(4))
		Expect(entries[0].Name).To(Equal("tomcat-access-logging-support"))
		Expect(entries[0].Build).To(BeFalse())
		Expect(entries[0].Launch).To(BeTrue())
		Expect(entries[1].Name).To(Equal("tomcat-lifecycle-support"))
		Expect(entries[1].Build).To(BeFalse())
		Expect(entries[1].Launch).To(BeTrue())
		Expect(entries[2].Name).To(Equal("tomcat-logging-support"))
		Expect(entries[2].Build).To(BeFalse())
		Expect(entries[2].Launch).To(BeTrue())
		Expect(entries[3].Name).To(Equal("tomcat-external-configuration"))
		Expect(entries[3].Build).To(BeFalse())
		Expect(entries[3].Launch).To(BeTrue())

		layer, err = contrib.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(filepath.Join(layer.Path, "fixture-marker")).To(BeARegularFile())
	})

	context("$BP_TOMCAT_EXT_CONF_STRIP", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_TOMCAT_EXT_CONF_STRIP", "1")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_TOMCAT_EXT_CONF_STRIP")).To(Succeed())
		})

		it("contributes custom configuration with directory", func() {
			externalConfigurationDep := libpak.BuildpackDependency{
				ID:     "tomcat-external-configuration",
				URI:    "https://localhost/stub-external-configuration-with-directory.tar.gz",
				SHA256: "060818cbcdc2008563f0f9e2428ecf4a199a5821c5b8b1dcd11a67666c1e2cd6",
				PURL:   "pkg:generic/tomcat@1.1.1",
				CPEs:   []string{"cpe:2.3:a:apache:tomcat:1.1.1:*:*:*:*:*:*:*"},
			}
			accessLoggingDep := libpak.BuildpackDependency{
				ID:     "tomcat-access-logging-support",
				URI:    "https://localhost/stub-tomcat-access-logging-support.jar",
				SHA256: "d723bfe2ba67dfa92b24e3b6c7b2d0e6a963de7313350e306d470e44e330a5d2",
				PURL:   "pkg:generic/tomcat-access-logging-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-access-logging-support:3.3.0:*:*:*:*:*:*:*"},
			}
			lifecycleDep := libpak.BuildpackDependency{
				ID:     "tomcat-lifecycle-support",
				URI:    "https://localhost/stub-tomcat-lifecycle-support.jar",
				SHA256: "723126712c0b22a7fe409664adf1fbb78cf3040e313a82c06696f5058e190534",
				PURL:   "pkg:generic/tomcat-lifecycle-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-lifecycle-support:3.3.0:*:*:*:*:*:*:*"},
			}
			loggingDep := libpak.BuildpackDependency{
				ID:     "tomcat-logging-support",
				URI:    "https://localhost/stub-tomcat-logging-support.jar",
				SHA256: "e0a7e163cc9f1ffd41c8de3942c7c6b505090b7484c2ba9be846334e31c44a2c",
				PURL:   "pkg:generic/tomcat-logging-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-logging-support:3.3.0:*:*:*:*:*:*:*"},
			}

			dc := libpak.DependencyCache{CachePath: "testdata"}

			contrib, entries := tomcat.NewBase(
				ctx.Application.Path,
				ctx.Buildpack.Path,
				libpak.ConfigurationResolver{},
				"test-context-path",
				accessLoggingDep,
				&externalConfigurationDep,
				lifecycleDep,
				loggingDep,
				dc,
				false,
			)
			Expect(entries).To(HaveLen(4))
			Expect(entries[0].Name).To(Equal("tomcat-access-logging-support"))
			Expect(entries[0].Build).To(BeFalse())
			Expect(entries[0].Launch).To(BeTrue())
			Expect(entries[1].Name).To(Equal("tomcat-lifecycle-support"))
			Expect(entries[1].Build).To(BeFalse())
			Expect(entries[1].Launch).To(BeTrue())
			Expect(entries[2].Name).To(Equal("tomcat-logging-support"))
			Expect(entries[2].Build).To(BeFalse())
			Expect(entries[2].Launch).To(BeTrue())
			Expect(entries[3].Name).To(Equal("tomcat-external-configuration"))
			Expect(entries[3].Build).To(BeFalse())
			Expect(entries[3].Launch).To(BeTrue())

			layer, err := ctx.Layers.Layer("test-layer")
			Expect(err).NotTo(HaveOccurred())

			layer, err = contrib.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			Expect(filepath.Join(layer.Path, "fixture-marker")).To(BeARegularFile())
		})
	})

	context("$BP_TOMCAT_ENV_PROPERTY_SOURCE_DISABLED", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_TOMCAT_ENV_PROPERTY_SOURCE_DISABLED", "true")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_TOMCAT_ENV_PROPERTY_SOURCE_DISABLED")).To(Succeed())
		})

		it("environment property source can be disabled", func() {
			accessLoggingDep := libpak.BuildpackDependency{
				ID:     "tomcat-access-logging-support",
				URI:    "https://localhost/stub-tomcat-access-logging-support.jar",
				SHA256: "d723bfe2ba67dfa92b24e3b6c7b2d0e6a963de7313350e306d470e44e330a5d2",
				PURL:   "pkg:generic/tomcat-access-logging-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-access-logging-support:3.3.0:*:*:*:*:*:*:*"},
			}
			lifecycleDep := libpak.BuildpackDependency{
				ID:     "tomcat-lifecycle-support",
				URI:    "https://localhost/stub-tomcat-lifecycle-support.jar",
				SHA256: "723126712c0b22a7fe409664adf1fbb78cf3040e313a82c06696f5058e190534",
				PURL:   "pkg:generic/tomcat-lifecycle-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-lifecycle-support:3.3.0:*:*:*:*:*:*:*"},
			}
			loggingDep := libpak.BuildpackDependency{
				ID:     "tomcat-logging-support",
				URI:    "https://localhost/stub-tomcat-logging-support.jar",
				SHA256: "e0a7e163cc9f1ffd41c8de3942c7c6b505090b7484c2ba9be846334e31c44a2c",
				PURL:   "pkg:generic/tomcat-logging-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-logging-support:3.3.0:*:*:*:*:*:*:*"},
			}

			dc := libpak.DependencyCache{CachePath: "testdata"}

			contributor, entries := tomcat.NewBase(
				ctx.Application.Path,
				ctx.Buildpack.Path,
				libpak.ConfigurationResolver{},
				"test-context-path",
				accessLoggingDep,
				nil,
				lifecycleDep,
				loggingDep,
				dc,
				false,
			)

			Expect(entries).To(HaveLen(3))
			Expect(entries[0].Name).To(Equal("tomcat-access-logging-support"))
			Expect(entries[0].Build).To(BeFalse())
			Expect(entries[0].Launch).To(BeTrue())
			Expect(entries[1].Name).To(Equal("tomcat-lifecycle-support"))
			Expect(entries[1].Build).To(BeFalse())
			Expect(entries[1].Launch).To(BeTrue())
			Expect(entries[2].Name).To(Equal("tomcat-logging-support"))
			Expect(entries[2].Build).To(BeFalse())
			Expect(entries[2].Launch).To(BeTrue())

			layer, err := ctx.Layers.Layer("test-layer")
			Expect(err).NotTo(HaveOccurred())

			layer, err = contributor.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			Expect(layer.Launch).To(BeTrue())
			Expect(filepath.Join(layer.Path, "conf", "context.xml")).To(BeARegularFile())
			Expect(filepath.Join(layer.Path, "conf", "logging.properties")).To(BeARegularFile())
			Expect(filepath.Join(layer.Path, "conf", "server.xml")).To(BeARegularFile())
			Expect(filepath.Join(layer.Path, "conf", "web.xml")).To(BeARegularFile())
			Expect(filepath.Join(layer.Path, "conf", "catalina.properties")).To(BeARegularFile())
			Expect(os.ReadFile(filepath.Join(layer.Path, "conf", "catalina.properties"))).To(ContainSubstring("common.loader=${BPI_TOMCAT_ADDITIONAL_COMMON_JARS}"))
			Expect(filepath.Join(layer.Path, "lib", "stub-tomcat-access-logging-support.jar")).To(BeARegularFile())
			Expect(filepath.Join(layer.Path, "lib", "stub-tomcat-lifecycle-support.jar")).To(BeARegularFile())
			Expect(filepath.Join(layer.Path, "bin", "stub-tomcat-logging-support.jar")).To(BeARegularFile())
			Expect(os.ReadFile(filepath.Join(layer.Path, "bin", "setenv.sh"))).To(Equal(
				[]byte(fmt.Sprintf(`CLASSPATH="%s"`, filepath.Join(layer.Path, "bin", "stub-tomcat-logging-support.jar")))))
			Expect(layer.LaunchEnvironment["CATALINA_BASE.default"]).To(Equal(layer.Path))
			Expect(filepath.Join(layer.Path, "temp")).To(BeADirectory())

			file := filepath.Join(layer.Path, "webapps", "test-context-path")
			fi, err := os.Lstat(file)
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.Mode() & os.ModeSymlink).To(Equal(os.ModeSymlink))
			Expect(os.Readlink(file)).To(Equal(ctx.Application.Path))

			Expect(layer.LaunchEnvironment["CATALINA_BASE.default"]).To(Equal(layer.Path))
			Expect(layer.LaunchEnvironment["CATALINA_OPTS.default"]).To(Equal("-DBPI_TOMCAT_ADDITIONAL_COMMON_JARS=${BPI_TOMCAT_ADDITIONAL_COMMON_JARS}"))
		})

	})

	context("$BPI_TOMCAT_ADDITIONAL_JARS is set", func() {
		it.Before(func() {
			t.Setenv("BPI_TOMCAT_ADDITIONAL_JARS", "/layers/test-buildpack/foo/bar.jar")
		})

		it("additional jar is added to classpath", func() {
			accessLoggingDep := libpak.BuildpackDependency{
				ID:     "tomcat-access-logging-support",
				URI:    "https://localhost/stub-tomcat-access-logging-support.jar",
				SHA256: "d723bfe2ba67dfa92b24e3b6c7b2d0e6a963de7313350e306d470e44e330a5d2",
				PURL:   "pkg:generic/tomcat-access-logging-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-access-logging-support:3.3.0:*:*:*:*:*:*:*"},
			}
			lifecycleDep := libpak.BuildpackDependency{
				ID:     "tomcat-lifecycle-support",
				URI:    "https://localhost/stub-tomcat-lifecycle-support.jar",
				SHA256: "723126712c0b22a7fe409664adf1fbb78cf3040e313a82c06696f5058e190534",
				PURL:   "pkg:generic/tomcat-lifecycle-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-lifecycle-support:3.3.0:*:*:*:*:*:*:*"},
			}
			loggingDep := libpak.BuildpackDependency{
				ID:     "tomcat-logging-support",
				URI:    "https://localhost/stub-tomcat-logging-support.jar",
				SHA256: "e0a7e163cc9f1ffd41c8de3942c7c6b505090b7484c2ba9be846334e31c44a2c",
				PURL:   "pkg:generic/tomcat-logging-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-logging-support:3.3.0:*:*:*:*:*:*:*"},
			}

			dc := libpak.DependencyCache{CachePath: "testdata"}

			contributor, entries := tomcat.NewBase(
				ctx.Application.Path,
				ctx.Buildpack.Path,
				libpak.ConfigurationResolver{},
				"test-context-path",
				accessLoggingDep,
				nil,
				lifecycleDep,
				loggingDep,
				dc,
				false,
			)

			Expect(entries).To(HaveLen(3))
			Expect(entries[0].Name).To(Equal("tomcat-access-logging-support"))
			Expect(entries[0].Build).To(BeFalse())
			Expect(entries[0].Launch).To(BeTrue())
			Expect(entries[1].Name).To(Equal("tomcat-lifecycle-support"))
			Expect(entries[1].Build).To(BeFalse())
			Expect(entries[1].Launch).To(BeTrue())
			Expect(entries[2].Name).To(Equal("tomcat-logging-support"))
			Expect(entries[2].Build).To(BeFalse())
			Expect(entries[2].Launch).To(BeTrue())

			layer, err := ctx.Layers.Layer("test-layer")
			Expect(err).NotTo(HaveOccurred())

			layer, err = contributor.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.ReadFile(filepath.Join(layer.Path, "bin", "setenv.sh"))).To(Equal(
				[]byte(fmt.Sprintf(`CLASSPATH="%s:%s"`, filepath.Join(layer.Path, "bin", "stub-tomcat-logging-support.jar"), "/layers/test-buildpack/foo/bar.jar"))))
		})

	})

	context("Contribute multiple war files", func() {
		files := []string{"api.war", "ui.war"}
		it.Before(func() {
			for _, file := range files {
				in, err := os.Open(filepath.Join("testdata", "warfiles", file))
				Expect(err).NotTo(HaveOccurred())

				out, err := os.OpenFile(filepath.Join(ctx.Application.Path, file), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Copy(out, in)
				Expect(err).NotTo(HaveOccurred())
				Expect(in.Close()).To(Succeed())
				Expect(out.Close()).To(Succeed())
			}
		})

		it.After(func() {
			for _, file := range files {
				os.Remove(filepath.Join(ctx.Application.Path, file))
			}
		})

		it("Multiple war files have been exploded in application path", func() {
			accessLoggingDep := libpak.BuildpackDependency{
				ID:     "tomcat-access-logging-support",
				URI:    "https://localhost/stub-tomcat-access-logging-support.jar",
				SHA256: "d723bfe2ba67dfa92b24e3b6c7b2d0e6a963de7313350e306d470e44e330a5d2",
				PURL:   "pkg:generic/tomcat-access-logging-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-access-logging-support:3.3.0:*:*:*:*:*:*:*"},
			}
			lifecycleDep := libpak.BuildpackDependency{
				ID:     "tomcat-lifecycle-support",
				URI:    "https://localhost/stub-tomcat-lifecycle-support.jar",
				SHA256: "723126712c0b22a7fe409664adf1fbb78cf3040e313a82c06696f5058e190534",
				PURL:   "pkg:generic/tomcat-lifecycle-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-lifecycle-support:3.3.0:*:*:*:*:*:*:*"},
			}
			loggingDep := libpak.BuildpackDependency{
				ID:     "tomcat-logging-support",
				URI:    "https://localhost/stub-tomcat-logging-support.jar",
				SHA256: "e0a7e163cc9f1ffd41c8de3942c7c6b505090b7484c2ba9be846334e31c44a2c",
				PURL:   "pkg:generic/tomcat-logging-support@3.3.0",
				CPEs:   []string{"cpe:2.3:a:cloudfoundry:tomcat-logging-support:3.3.0:*:*:*:*:*:*:*"},
			}

			dc := libpak.DependencyCache{CachePath: "testdata"}

			contributor, entries := tomcat.NewBase(
				ctx.Application.Path,
				ctx.Buildpack.Path,
				libpak.ConfigurationResolver{},
				"test-context-path",
				accessLoggingDep,
				nil,
				lifecycleDep,
				loggingDep,
				dc,
				true,
			)

			Expect(entries).To(HaveLen(3))
			Expect(entries[0].Name).To(Equal("tomcat-access-logging-support"))
			Expect(entries[0].Build).To(BeFalse())
			Expect(entries[0].Launch).To(BeTrue())
			Expect(entries[1].Name).To(Equal("tomcat-lifecycle-support"))
			Expect(entries[1].Build).To(BeFalse())
			Expect(entries[1].Launch).To(BeTrue())
			Expect(entries[2].Name).To(Equal("tomcat-logging-support"))
			Expect(entries[2].Build).To(BeFalse())
			Expect(entries[2].Launch).To(BeTrue())

			layer, err := ctx.Layers.Layer("test-layer")
			Expect(err).NotTo(HaveOccurred())

			layer, err = contributor.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.Readlink(filepath.Join(layer.Path, "webapps"))).To(Equal(ctx.Application.Path))
			for _, file := range files {
				targetDir := strings.TrimSuffix(file, filepath.Ext(file))
				Expect(filepath.Join(layer.Path, "webapps", targetDir, "META-INF", "MANIFEST.MF")).To(BeARegularFile())
			}
		})
	})

}
