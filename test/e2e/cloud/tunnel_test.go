// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cloud_test

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:stylecheck
	. "github.com/onsi/gomega"    //nolint:stylecheck

	fcmd "kraftkit.sh/test/e2e/framework/cmd"
	fcfg "kraftkit.sh/test/e2e/framework/config"
)

func parsePrivateIP(stdout *fcmd.IOStream) string {
	if strings.Contains(stdout.String(), "\"private_ip\"") {
		ip := strings.SplitN(stdout.String(), "private_ip\":\"", 2)[1]
		ip = strings.SplitN(ip, "\"", 2)[0]
		if ip == "" {
			return ""
		}
		return ip
	}

	return ""
}

var _ = Describe("kraft cloud tunnel", func() {
	var cmd *fcmd.Cmd

	var stdout *fcmd.IOStream
	var stderr *fcmd.IOStream

	var cfg *fcfg.Config

	const (
		imageName      = "nginx:latest"
		instanceName   = "instance-tunnel-test"
		instanceMemory = "64"
		internalPort   = "8080"
		localPortStart = 8000
	)

	BeforeEach(func() {
		token := os.Getenv("UNIKRAFTCLOUD_TOKEN")

		if token == "" {
			token = os.Getenv("KRAFTCLOUD_TOKEN")
		}

		if token == "" {
			token = os.Getenv("KC_TOKEN")
		}

		if token == "" {
			token = os.Getenv("UKC_TOKEN")
		}

		if token == "" {
			Skip("UNIKRAFTCLOUD_TOKEN is not set")
		}

		metro := os.Getenv("UNIKRAFTCLOUD_METRO")

		if metro == "" {
			metro = os.Getenv("KRAFTCLOUD_METRO")
		}

		if metro == "" {
			metro = os.Getenv("KC_METRO")
		}

		if metro == "" {
			metro = os.Getenv("UKC_METRO")
		}

		if metro == "" {
			Skip("UNIKRAFTCLOUD_METRO is not set")
		}

		stdout = fcmd.NewIOStream()
		stderr = fcmd.NewIOStream()

		cfg = fcfg.NewTempConfig()

		cmd = fcmd.NewKraft(stdout, stderr, cfg.Path())
		cmd.Env = os.Environ()
		cmd.Args = append(cmd.Args, "cloud", "tunnel", "--log-level", "info", "--log-type", "json")
	})

	When("invoked with a single, contracted, instance", Serial, func() {
		var instanceNameFull string
		var localPort string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)

			randPort, err := rand.Int(rand.Reader, big.NewInt(999))
			if err != nil {
				panic(err)
			}
			localPort = fmt.Sprintf("%d", localPortStart+int(randPort.Int64()))

			instanceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--start",
				imageName)

			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			err := cmd.Process.Kill()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Process.Release()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			instanceRemoveCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRemoveCmd.Env = os.Environ()
			instanceRemoveCmd.Args = append(instanceRemoveCmd.Args, "cloud", "instance", "remove", instanceNameFull)

			err = instanceRemoveCmd.Run()
			if err != nil {
				fmt.Print(instanceRemoveCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should allow connection to the instance", func() {
			startCurl := make(chan struct{}, 1)
			go func() {
				cmd.Args = append(cmd.Args, localPort+":"+instanceNameFull+":"+internalPort)
				startCurl <- struct{}{}
				_ = cmd.Run()
			}()

			<-startCurl
			time.Sleep(time.Second)

			// Run the "curl" command to test the url
			curlCmd := fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, "http://localhost:"+localPort)

			err := curlCmd.Run()
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Welcome to nginx!`))
		})
	})

	When("invoked with a single, uncontracted, instance", Serial, func() {
		var instanceNameFull string
		var localPort string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)

			randPort, err := rand.Int(rand.Reader, big.NewInt(999))
			if err != nil {
				panic(err)
			}
			localPort = fmt.Sprintf("%d", localPortStart+int(randPort.Int64()))

			instanceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--start",
				imageName)

			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			err := cmd.Process.Kill()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Process.Release()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			instanceRemoveCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRemoveCmd.Env = os.Environ()
			instanceRemoveCmd.Args = append(instanceRemoveCmd.Args, "cloud", "instance", "remove", instanceNameFull)

			err = instanceRemoveCmd.Run()
			if err != nil {
				fmt.Print(instanceRemoveCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should allow connection to the instance", func() {
			startCurl := make(chan struct{}, 1)
			go func() {
				cmd.Args = append(cmd.Args, localPort+":"+instanceNameFull+":"+internalPort+"/tcp")
				startCurl <- struct{}{}
				_ = cmd.Run()
			}()

			<-startCurl
			time.Sleep(time.Second)

			// Run the "curl" command to test the url
			curlCmd := fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, "http://localhost:"+localPort)

			err := curlCmd.Run()
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Welcome to nginx!`))
		})
	})

	When("invoked with a single, contracted, internal FQDN", Serial, func() {
		var instanceNameFull string
		var localPort string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)

			randPort, err := rand.Int(rand.Reader, big.NewInt(999))
			if err != nil {
				panic(err)
			}
			localPort = fmt.Sprintf("%d", localPortStart+int(randPort.Int64()))

			instanceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--start",
				imageName)

			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			err := cmd.Process.Kill()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Process.Release()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			instanceRemoveCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRemoveCmd.Env = os.Environ()
			instanceRemoveCmd.Args = append(instanceRemoveCmd.Args, "cloud", "instance", "remove", instanceNameFull)

			err = instanceRemoveCmd.Run()
			if err != nil {
				fmt.Print(instanceRemoveCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should allow connection to the instance", func() {
			startCurl := make(chan struct{}, 1)
			go func() {
				cmd.Args = append(cmd.Args, localPort+":"+instanceNameFull+".internal"+":"+internalPort)
				startCurl <- struct{}{}
				_ = cmd.Run()
			}()

			<-startCurl
			time.Sleep(time.Second)

			// Run the "curl" command to test the url
			curlCmd := fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, "http://localhost:"+localPort)

			err := curlCmd.Run()
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Welcome to nginx!`))
		})
	})

	When("invoked with a single, contracted, private ip", Serial, func() {
		var instanceNameFull string
		var instancePrivateIP string
		var localPort string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)

			randPort, err := rand.Int(rand.Reader, big.NewInt(999))
			if err != nil {
				panic(err)
			}
			localPort = fmt.Sprintf("%d", localPortStart+int(randPort.Int64()))

			instanceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--start",
				imageName)

			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			instancePrivateIP = parsePrivateIP(stdout)
			Expect(instancePrivateIP).ToNot(BeEmpty())
		})

		AfterEach(func() {
			err := cmd.Process.Kill()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Process.Release()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			instanceRemoveCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRemoveCmd.Env = os.Environ()
			instanceRemoveCmd.Args = append(instanceRemoveCmd.Args, "cloud", "instance", "remove", instanceNameFull)

			err = instanceRemoveCmd.Run()
			if err != nil {
				fmt.Print(instanceRemoveCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should allow connection to the instance", func() {
			startCurl := make(chan struct{}, 1)
			go func() {
				cmd.Args = append(cmd.Args, localPort+":"+instancePrivateIP+":"+internalPort)
				startCurl <- struct{}{}
				_ = cmd.Run()
			}()

			<-startCurl
			time.Sleep(time.Second)

			// Run the "curl" command to test the url
			curlCmd := fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, "http://localhost:"+localPort)

			err := curlCmd.Run()
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Welcome to nginx!`))
		})
	})

	When("invoked with two, uncontracted, instances", Serial, func() {
		var instanceNameFull1 string
		var localPort1 string
		var instanceNameFull2 string
		var localPort2 string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull1 = fmt.Sprintf("%s-%d-1", instanceName, id)
			instanceNameFull2 = fmt.Sprintf("%s-%d-2", instanceName, id)

			randPort, err := rand.Int(rand.Reader, big.NewInt(999))
			if err != nil {
				panic(err)
			}
			localPort1 = fmt.Sprintf("%d", localPortStart+int(randPort.Int64()))

			randPort, err = rand.Int(rand.Reader, big.NewInt(999))
			if err != nil {
				panic(err)
			}
			localPort2 = fmt.Sprintf("%d", localPortStart+int(randPort.Int64()))

			instanceCreateCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd1.Env = os.Environ()
			instanceCreateCmd1.Args = append(instanceCreateCmd1.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", instanceNameFull1,
				"--memory", instanceMemory,
				"--start",
				imageName)

			err = instanceCreateCmd1.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			instanceCreateCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd2.Env = os.Environ()
			instanceCreateCmd2.Args = append(instanceCreateCmd2.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", instanceNameFull2,
				"--memory", instanceMemory,
				"--start",
				imageName)

			err = instanceCreateCmd2.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			err := cmd.Process.Kill()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Process.Release()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			instanceRemoveCmd1 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRemoveCmd1.Env = os.Environ()
			instanceRemoveCmd1.Args = append(instanceRemoveCmd1.Args, "cloud", "instance", "remove", instanceNameFull1)

			err = instanceRemoveCmd1.Run()
			if err != nil {
				fmt.Print(instanceRemoveCmd1.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())

			instanceRemoveCmd2 := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRemoveCmd2.Env = os.Environ()
			instanceRemoveCmd2.Args = append(instanceRemoveCmd2.Args, "cloud", "instance", "remove", instanceNameFull2)

			err = instanceRemoveCmd2.Run()
			if err != nil {
				fmt.Print(instanceRemoveCmd2.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should allow connection to the instance", func() {
			startCurl := make(chan struct{}, 1)
			go func() {
				cmd.Args = append(cmd.Args,
					localPort1+":"+instanceNameFull1+":"+internalPort+"/tcp",
					localPort2+":"+instanceNameFull2+":"+internalPort+"/tcp")
				startCurl <- struct{}{}
				_ = cmd.Run()
			}()

			<-startCurl
			time.Sleep(time.Second)

			// Run the "curl" command to test the url
			curlCmd := fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, "http://localhost:"+localPort1)

			err := curlCmd.Run()
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Welcome to nginx!`))

			// Run the "curl" command to test the url
			curlCmd = fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, "http://localhost:"+localPort2)

			err = curlCmd.Run()
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Welcome to nginx!`))
		})
	})

	When("invoked with one, uncontracted, instance and custom control ports", Serial, func() {
		var instanceNameFull string
		var localPort string

		BeforeEach(func() {
			id, err := rand.Int(rand.Reader, big.NewInt(100000000000))
			if err != nil {
				panic(err)
			}
			instanceNameFull = fmt.Sprintf("%s-%d", instanceName, id)

			randPort, err := rand.Int(rand.Reader, big.NewInt(999))
			if err != nil {
				panic(err)
			}
			localPort = fmt.Sprintf("%d", localPortStart+int(randPort.Int64()))

			instanceCreateCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceCreateCmd.Env = os.Environ()
			instanceCreateCmd.Args = append(instanceCreateCmd.Args, "cloud", "instance", "create",
				"--log-level", "info", "--log-type", "json", "-o", "json",
				"--name", instanceNameFull,
				"--memory", instanceMemory,
				"--start",
				imageName)

			err = instanceCreateCmd.Run()
			if err != nil {
				fmt.Print(instanceCreateCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		AfterEach(func() {
			err := cmd.Process.Kill()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			err = cmd.Process.Release()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			instanceRemoveCmd := fcmd.NewKraft(stdout, stderr, cfg.Path())
			instanceRemoveCmd.Env = os.Environ()
			instanceRemoveCmd.Args = append(instanceRemoveCmd.Args, "cloud", "instance", "remove", instanceNameFull)

			err = instanceRemoveCmd.Run()
			if err != nil {
				fmt.Print(instanceRemoveCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
		})

		It("should allow connection to the instance", func() {
			startCurl := make(chan struct{}, 1)
			go func() {
				cmd.Args = append(cmd.Args,
					"--tunnel-control-port", "5443",
					"--tunnel-proxy-port", "5444",
					localPort+":"+instanceNameFull+":"+internalPort+"/tcp")
				startCurl <- struct{}{}
				_ = cmd.Run()
			}()

			<-startCurl
			time.Sleep(time.Second)

			// Run the "curl" command to test the url
			curlCmd := fcmd.NewCurl(stdout, stderr)
			curlCmd.Args = append(curlCmd.Args, "http://localhost:"+localPort)

			err := curlCmd.Run()
			if err != nil {
				fmt.Print(curlCmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).ToNot(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Welcome to nginx!`))
		})
	})

	When("invoked with the --help flag", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "--help")
		})

		It("should print the command's help", func() {
			err := cmd.Run()
			if err != nil {
				fmt.Print(cmd.DumpError(stdout, stderr, err))
			}
			Expect(err).ToNot(HaveOccurred())

			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(MatchRegexp(`Forward a local port to an unexposed instance`))
		})
	})
})
