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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/apache-tomcat/tomcat"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx libcnb.BuildContext
	)

	it.Before(func() {
		var err error
		ctx.Application.Path, err = ioutil.TempDir("", "tomcat-application")
		Expect(err).NotTo(HaveOccurred())
		ctx.Plan = libcnb.BuildpackPlan{Entries: []libcnb.BuildpackPlanEntry{
			{Name: "jvm-application"},
		}}
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
	})

	it("does not contribute Tomcat if no WEB-INF", func() {
		result, err := tomcat.Build{}.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(BeEmpty())
		Expect(result.Unmet).To(HaveLen(1))
		Expect(result.Unmet[0].Name).To(Equal("jvm-application"))
	})

	it("does not contribute Tomcat if Main-Class", func() {
		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "META-INF"), 0755)).To(Succeed())
		Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte(`Main-Class: test-main-class`), 0644)).To(Succeed())

		result, err := tomcat.Build{}.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(BeEmpty())
		Expect(result.Unmet).To(HaveLen(1))
		Expect(result.Unmet[0].Name).To(Equal("jvm-application"))
	})

	it("contributes Tomcat", func() {
		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"dependencies": []map[string]interface{}{
				{
					"id":      "tomcat",
					"version": "1.1.1",
					"stacks":  []interface{}{"test-stack-id"},
				},
				{
					"id":      "tomcat-access-logging-support",
					"version": "1.1.1",
					"stacks":  []interface{}{"test-stack-id"},
				},
				{
					"id":      "tomcat-lifecycle-support",
					"version": "1.1.1",
					"stacks":  []interface{}{"test-stack-id"},
				},
				{
					"id":      "tomcat-logging-support",
					"version": "1.1.1",
					"stacks":  []interface{}{"test-stack-id"},
				},
			},
		}
		ctx.StackID = "test-stack-id"

		result, err := tomcat.Build{}.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Processes).To(ContainElements(
			libcnb.Process{Type: "task", Command: "bash", Arguments: []string{"catalina.sh", "run"}, Direct: true},
			libcnb.Process{Type: "tomcat", Command: "bash", Arguments: []string{"catalina.sh", "run"}, Direct: true},
			libcnb.Process{Type: "web", Command: "bash", Arguments: []string{"catalina.sh", "run"}, Direct: true, Default: true},
		))

		Expect(result.Layers).To(HaveLen(3))
		Expect(result.Layers[0].Name()).To(Equal("tomcat"))
		Expect(result.Layers[1].Name()).To(Equal("helper"))
		Expect(result.Layers[1].(libpak.HelperLayerContributor).Names).To(Equal([]string{"access-logging-support"}))
		Expect(result.Layers[2].Name()).To(Equal("catalina-base"))

		Expect(result.BOM.Entries).To(HaveLen(5))
		Expect(result.BOM.Entries[0].Name).To(Equal("tomcat"))
		Expect(result.BOM.Entries[1].Name).To(Equal("helper"))
		Expect(result.BOM.Entries[2].Name).To(Equal("tomcat-access-logging-support"))
		Expect(result.BOM.Entries[3].Name).To(Equal("tomcat-lifecycle-support"))
		Expect(result.BOM.Entries[4].Name).To(Equal("tomcat-logging-support"))
	})

	it("contributes Tomcat on Tiny", func() {
		ctx.StackID = libpak.TinyStackID

		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"dependencies": []map[string]interface{}{
				{
					"id":      "tomcat",
					"version": "1.1.1",
					"stacks":  []interface{}{libpak.TinyStackID},
				},
				{
					"id":      "tomcat-access-logging-support",
					"version": "1.1.1",
					"stacks":  []interface{}{libpak.TinyStackID},
				},
				{
					"id":      "tomcat-lifecycle-support",
					"version": "1.1.1",
					"stacks":  []interface{}{libpak.TinyStackID},
				},
				{
					"id":      "tomcat-logging-support",
					"version": "1.1.1",
					"uri":     "https://example.com/releases/tomcat-logging-support-1.1.1.RELEASE.jar",
					"stacks":  []interface{}{libpak.TinyStackID},
				},
			},
		}

		result, err := tomcat.Build{}.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		for _, procType := range []string{"task", "tomcat", "web"} {
			expectedProcess := libcnb.Process{
				Type:    procType,
				Command: "java",
				Arguments: []string{
					"-Djava.util.logging.config.file=catalina-base/conf/logging.properties",
					"-Djava.util.logging.manager=org.apache.juli.ClassLoaderLogManager",
					"-Djdk.tls.ephemeralDHKeySize=2048",
					"-classpath",
					"catalina-base/bin/tomcat-logging-support-1.1.1.RELEASE.jar:tomcat/bin/bootstrap.jar:tomcat/bin/tomcat-juli.jar",
					"-Dcatalina.home=tomcat",
					"-Dcatalina.base=catalina-base",
					"-Djava.io.tmpdir=catalina-base/temp",
					"org.apache.catalina.startup.Bootstrap",
					"start",
				},
				Direct: true,
			}
			if procType == "web" {
				expectedProcess.Default = true
			}
			Expect(result.Processes).To(ContainElement(expectedProcess))
		}

		Expect(result.Layers).To(HaveLen(3))
		Expect(result.Layers[0].Name()).To(Equal("tomcat"))
		Expect(result.Layers[1].Name()).To(Equal("helper"))
		Expect(result.Layers[1].(libpak.HelperLayerContributor).Names).To(Equal([]string{"access-logging-support"}))
		Expect(result.Layers[2].Name()).To(Equal("catalina-base"))

		Expect(result.BOM.Entries).To(HaveLen(5))
		Expect(result.BOM.Entries[0].Name).To(Equal("tomcat"))
		Expect(result.BOM.Entries[1].Name).To(Equal("helper"))
		Expect(result.BOM.Entries[2].Name).To(Equal("tomcat-access-logging-support"))
		Expect(result.BOM.Entries[3].Name).To(Equal("tomcat-lifecycle-support"))
		Expect(result.BOM.Entries[4].Name).To(Equal("tomcat-logging-support"))
	})

	context("$BP_TOMCAT_VERSION", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_TOMCAT_VERSION", "1.1.1")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_TOMCAT_VERSION")).To(Succeed())
		})

		it("selects version based on $BP_TOMCAT_VERSION", func() {
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())

			ctx.Buildpack.Metadata = map[string]interface{}{
				"dependencies": []map[string]interface{}{
					{
						"id":      "tomcat",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
					},
					{
						"id":      "tomcat",
						"version": "2.2.2",
						"stacks":  []interface{}{"test-stack-id"},
					},
					{
						"id":      "tomcat-access-logging-support",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
					},
					{
						"id":      "tomcat-lifecycle-support",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
					},
					{
						"id":      "tomcat-logging-support",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
					},
				},
			}
			ctx.StackID = "test-stack-id"

			result, err := tomcat.Build{}.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers[0].(tomcat.Home).LayerContributor.Dependency.Version).To(Equal("1.1.1"))
		})
	})

	context("$BP_TOMCAT_EXT_CONF_URI", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_TOMCAT_EXT_CONF_SHA256", "test-sha256")).To(Succeed())
			Expect(os.Setenv("BP_TOMCAT_EXT_CONF_URI", "test-uri")).To(Succeed())
			Expect(os.Setenv("BP_TOMCAT_EXT_CONF_VERSION", "test-version")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_TOMCAT_EXT_CONF_SHA256")).To(Succeed())
			Expect(os.Unsetenv("BP_TOMCAT_EXT_CONF_URI")).To(Succeed())
			Expect(os.Unsetenv("BP_TOMCAT_EXT_CONF_VERSION")).To(Succeed())
		})

		it("contributes external configuration when $BP_TOMCAT_EXT_CONF_URI is set", func() {
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())

			ctx.Buildpack.Metadata = map[string]interface{}{
				"dependencies": []map[string]interface{}{
					{
						"id":      "tomcat",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
					},
					{
						"id":      "tomcat-access-logging-support",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
					},
					{
						"id":      "tomcat-lifecycle-support",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
					},
					{
						"id":      "tomcat-logging-support",
						"version": "1.1.1",
						"stacks":  []interface{}{"test-stack-id"},
					},
				},
			}
			ctx.StackID = "test-stack-id"

			result, err := tomcat.Build{}.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers[2].(tomcat.Base).ExternalConfigurationDependency).To(Equal(&libpak.BuildpackDependency{
				ID:      "tomcat-external-configuration",
				Name:    "Tomcat External Configuration",
				Version: "test-version",
				URI:     "test-uri",
				SHA256:  "test-sha256",
				Stacks:  []string{ctx.StackID},
			}))
		})
	})

	it("returns default context path", func() {
		Expect(tomcat.Build{}.ContextPath(libpak.ConfigurationResolver{})).To(Equal("ROOT"))
	})

	context("$BP_TOMCAT_CONTEXT_PATH", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_TOMCAT_CONTEXT_PATH", "/alpha/bravo/")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_TOMCAT_CONTEXT_PATH")).To(Succeed())
		})

		it("returns transformed context path", func() {
			Expect(tomcat.Build{}.ContextPath(libpak.ConfigurationResolver{})).To(Equal("alpha#bravo"))
		})
	})
}
