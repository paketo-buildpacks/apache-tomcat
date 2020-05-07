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
	"github.com/paketo-buildpacks/apache-tomcat/tomcat"
	"github.com/paketo-buildpacks/libpak"
	"github.com/sclevine/spec"
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

	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
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
			libcnb.Process{Type: "task", Command: "catalina.sh run"},
			libcnb.Process{Type: "tomcat", Command: "catalina.sh run"},
			libcnb.Process{Type: "web", Command: "catalina.sh run"},
		))

		Expect(result.Layers).To(HaveLen(2))
		Expect(result.Layers[0].Name()).To(Equal("catalina-home"))
		Expect(result.Layers[1].Name()).To(Equal("catalina-base"))
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

			Expect(result.Layers[1].(tomcat.Base).ExternalConfigurationDependency).To(Equal(&libpak.BuildpackDependency{
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
