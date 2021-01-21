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

package native_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/magiconair/properties"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"

	"github.com/paketo-buildpacks/spring-boot-native-image/native"
)

func testNativeImage(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx         libcnb.BuildContext
		executor    *mocks.Executor
		props       *properties.Properties
		nativeImage native.NativeImage
		layer       libcnb.Layer
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "native-image-application")
		Expect(err).NotTo(HaveOccurred())

		ctx.Layers.Path, err = ioutil.TempDir("", "native-image-layers")
		Expect(err).NotTo(HaveOccurred())

		executor = &mocks.Executor{}

		props = properties.NewProperties()

		_, _, err = props.Set("Start-Class", "test-start-class")
		Expect(err).NotTo(HaveOccurred())
		_, _, err = props.Set("Spring-Boot-Classes", "BOOT-INF/classes/")
		Expect(err).NotTo(HaveOccurred())
		_, _, err = props.Set("Spring-Boot-Classpath-Index", "BOOT-INF/classpath.idx")
		Expect(err).NotTo(HaveOccurred())
		_, _, err = props.Set("Spring-Boot-Lib", "BOOT-INF/lib/")
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "fixture-marker"), []byte{}, 0644)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "BOOT-INF"), 0755)).To(Succeed())

		nativeImage, err = native.NewNativeImage(ctx.Application.Path, "test-argument-1 test-argument-2", props, ctx.StackID)
		Expect(err).NotTo(HaveOccurred())
		nativeImage.Executor = executor

		executor.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
			Expect(ioutil.WriteFile(filepath.Join(layer.Path, "test-start-class"), []byte{}, 0644)).To(Succeed())
		}).Return(nil)

		layer, err = ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	context("classpath.idx contains a list of jar", func() {
		context("neither spring-native nor spring-graalvm-native dependency", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "BOOT-INF", "classpath.idx"), []byte(`
- "test-jar.jar"`), 0644)).To(Succeed())
			})

			it("fails", func() {
				_, err := nativeImage.Contribute(layer)
				Expect(err).To(HaveOccurred())
			})
		})

		context("spring-native dependency", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "BOOT-INF", "classpath.idx"), []byte(`
- "test-jar.jar"
- "spring-native-0.8.6-xxxxxx.jar"
`), 0644)).To(Succeed())
			})

			it("contributes native image", func() {
				_, err := nativeImage.Contribute(layer)
				Expect(err).NotTo(HaveOccurred())

				execution := executor.Calls[1].Arguments[0].(effect.Execution)
				Expect(execution.Args).To(Equal([]string{
					"test-argument-1",
					"test-argument-2",
					fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-start-class")),
					"-cp",
					strings.Join([]string{
						filepath.Join(ctx.Application.Path),
						filepath.Join(ctx.Application.Path, "BOOT-INF", "classes"),
						filepath.Join(ctx.Application.Path, "BOOT-INF", "lib", "test-jar.jar"),
						filepath.Join(ctx.Application.Path, "BOOT-INF", "lib", "spring-native-0.8.6-xxxxxx.jar"),
					}, ":"),
					"test-start-class",
				}))
			})
		})

		context("spring-graalvm-native dependency", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "BOOT-INF", "classpath.idx"), []byte(`
- "test-jar.jar"
- "spring-graalvm-native-0.8.0-20200729.130845-95.jar"
`), 0644)).To(Succeed())
			})

			it("contributes native image", func() {
				_, err := nativeImage.Contribute(layer)
				Expect(err).NotTo(HaveOccurred())

				execution := executor.Calls[1].Arguments[0].(effect.Execution)
				Expect(execution.Args).To(Equal([]string{
					"test-argument-1",
					"test-argument-2",
					fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-start-class")),
					"-cp",
					strings.Join([]string{
						filepath.Join(ctx.Application.Path),
						filepath.Join(ctx.Application.Path, "BOOT-INF", "classes"),
						filepath.Join(ctx.Application.Path, "BOOT-INF", "lib", "test-jar.jar"),
						filepath.Join(ctx.Application.Path, "BOOT-INF", "lib", "spring-graalvm-native-0.8.0-20200729.130845-95.jar"),
					}, ":"),
					"test-start-class",
				}))
			})
		})
	})

	context("classpath.idx contains a list of relative paths to jar", func() {
		context("has neither spring-native nor spring-graalvm-native dependency", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "BOOT-INF", "classpath.idx"), []byte(`
- "test-jar.jar"`), 0644)).To(Succeed())
			})

			it("fails", func() {
				_, err := nativeImage.Contribute(layer)
				Expect(err).To(HaveOccurred())
			})
		})

		context("spring-native dependency", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "BOOT-INF", "classpath.idx"), []byte(`
- "some/path/test-jar.jar"
- "some/path/spring-native-0.8.6-xxxxxx.jar"
`), 0644)).To(Succeed())
			})

			it("contributes native image", func() {
				_, err := nativeImage.Contribute(layer)
				Expect(err).NotTo(HaveOccurred())

				execution := executor.Calls[1].Arguments[0].(effect.Execution)
				Expect(execution.Args).To(Equal([]string{
					"test-argument-1",
					"test-argument-2",
					fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-start-class")),
					"-cp",
					strings.Join([]string{
						filepath.Join(ctx.Application.Path),
						filepath.Join(ctx.Application.Path, "BOOT-INF", "classes"),
						filepath.Join(ctx.Application.Path, "some", "path", "test-jar.jar"),
						filepath.Join(ctx.Application.Path, "some", "path", "spring-native-0.8.6-xxxxxx.jar"),
					}, ":"),
					"test-start-class",
				}))
			})
		})

		context("spring-graalvm-native dependency", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "BOOT-INF", "classpath.idx"), []byte(`
- "some/path/test-jar.jar"
- "some/path/spring-graalvm-native-0.8.0-20200729.130845-95.jar"
`), 0644)).To(Succeed())
			})

			it("contributes native image", func() {
				_, err := nativeImage.Contribute(layer)
				Expect(err).NotTo(HaveOccurred())

				execution := executor.Calls[1].Arguments[0].(effect.Execution)
				Expect(execution.Args).To(Equal([]string{
					"test-argument-1",
					"test-argument-2",
					fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-start-class")),
					"-cp",
					strings.Join([]string{
						filepath.Join(ctx.Application.Path),
						filepath.Join(ctx.Application.Path, "BOOT-INF", "classes"),
						filepath.Join(ctx.Application.Path, "some", "path", "test-jar.jar"),
						filepath.Join(ctx.Application.Path, "some", "path", "spring-graalvm-native-0.8.0-20200729.130845-95.jar"),
					}, ":"),
					"test-start-class",
				}))
			})
		})
	})

	context("tiny stack", func() {
		it.Before(func() {
			nativeImage.StackID = libpak.TinyStackID
		})

		it("contributes native image", func() {
			Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "BOOT-INF", "classpath.idx"), []byte(`
- "test-jar.jar"
- "spring-graalvm-native-0.8.6-xxxxxx.jar"
`), 0644)).To(Succeed())
			var err error
			layer, err := nativeImage.Contribute(layer)
			Expect(err).NotTo(HaveOccurred())

			Expect(layer.Cache).To(BeTrue())
			Expect(filepath.Join(layer.Path, "test-start-class")).To(BeARegularFile())
			Expect(filepath.Join(ctx.Application.Path, "test-start-class")).To(BeARegularFile())
			Expect(filepath.Join(ctx.Application.Path, "fixture-marker")).NotTo(BeAnExistingFile())

			execution := executor.Calls[1].Arguments[0].(effect.Execution)
			Expect(execution.Command).To(Equal("native-image"))
			Expect(execution.Args).To(Equal([]string{
				"test-argument-1",
				"test-argument-2",
				"-H:+StaticExecutableWithDynamicLibC",
				fmt.Sprintf("-H:Name=%s", filepath.Join(layer.Path, "test-start-class")),
				"-cp",
				strings.Join([]string{
					filepath.Join(ctx.Application.Path),
					filepath.Join(ctx.Application.Path, "BOOT-INF", "classes"),
					filepath.Join(ctx.Application.Path, "BOOT-INF", "lib", "test-jar.jar"),
					filepath.Join(ctx.Application.Path, "BOOT-INF", "lib", "spring-graalvm-native-0.8.6-xxxxxx.jar"),
				}, ":"),
				"test-start-class",
			}))
			Expect(execution.Dir).To(Equal(layer.Path))
		})
	})
}
