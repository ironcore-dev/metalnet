// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package sysfs_test

import (
	"os"
	"path/filepath"

	. "github.com/ironcore-dev/metalnet/encoding/sysfs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sysfs", func() {
	Describe("Unmarshal", func() {
		var (
			dir1           string
			dir1UInt64File string
		)
		BeforeEach(func() {
			dir1 = filepath.Join(GinkgoT().TempDir(), "dir1")
			Expect(os.MkdirAll(dir1, 0777)).To(Succeed())

			dir1UInt64File = filepath.Join(dir1, "uint64")
			Expect(os.WriteFile(dir1UInt64File, []byte("123"), 0666)).To(Succeed())
		})

		type dir1Struct struct {
			V1 uint64 `sysfs:"uint64"`
		}

		It("should unmarshal a directory with a uint value", func() {
			s := &dir1Struct{}
			Expect(Unmarshal(dir1, s)).To(Succeed())

			Expect(s).To(Equal(&dir1Struct{
				V1: 123,
			}))
		})
	})
})
